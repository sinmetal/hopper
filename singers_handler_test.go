package hopper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	"google.golang.org/api/iterator"
	adminpb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
)

const (
	testProjectID  = "test-project"
	testInstanceID = "test-instance"
	testDatabaseID = "test-db"
)

func setupTestDB(t *testing.T) *spanner.Client {
	t.Helper()

	emulatorHost := os.Getenv("SPANNER_EMULATOR_HOST")
	if emulatorHost == "" {
		t.Logf("SPANNER_EMULATOR_HOST environment variable is not set. Skipping tests that require Spanner Emulator.")
		t.Skipf("SPANNER_EMULATOR_HOST not set, skipping %s", t.Name())
		return nil
	}

	ctx := context.Background()
	dbPath := fmt.Sprintf("projects/%s/instances/%s/databases/%s", testProjectID, testInstanceID, testDatabaseID)

	dbAdminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		log.Fatalf("failed to create DatabaseAdminClient: %v", err)
	}
	defer dbAdminClient.Close()

	// Drop the database if it exists, to ensure a clean state
	_ = dbAdminClient.DropDatabase(ctx, &adminpb.DropDatabaseRequest{Database: dbPath})

	op, err := dbAdminClient.CreateDatabase(ctx, &adminpb.CreateDatabaseRequest{
		Parent:          fmt.Sprintf("projects/%s/instances/%s", testProjectID, testInstanceID),
		CreateStatement: "CREATE DATABASE `" + testDatabaseID + "`",
		ExtraStatements: []string{
			`CREATE TABLE Singers (
				SingerID STRING(MAX) NOT NULL,
				FirstName STRING(1024),
				LastName STRING(1024),
				CreatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE),
				UpdatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE)
			) PRIMARY KEY (SingerID)`,
			`CREATE TABLE Albums (
				SingerID STRING(MAX) NOT NULL,
				AlbumID STRING(MAX) NOT NULL,
				AlbumTitle STRING(MAX),
				Price INT64 NOT NULL,
				CreatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE),
				UpdatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE)
			) PRIMARY KEY (SingerID, AlbumID),
			INTERLEAVE IN PARENT Singers ON DELETE CASCADE`,
			// Removed locality group options as they might cause issues with emulator or are not needed for these tests
		},
	})
	if err != nil {
		// It's possible that a previous timed-out run left the DB in a partially created state or similar.
		// Log the error and attempt to continue if it's "AlreadyExists", otherwise fail hard.
		if !strings.Contains(err.Error(), "AlreadyExists") && !strings.Contains(err.Error(), "Duplicate") { // "Duplicate" for table already exists
			log.Fatalf("failed to create database: %v", err)
		}
	} else {
		if _, err := op.Wait(ctx); err != nil {
			if !strings.Contains(err.Error(), "AlreadyExists") {
				log.Fatalf("failed to wait for database creation: %v", err)
			}
		}
	}

	spannerClient, err := spanner.NewClient(ctx, dbPath)
	if err != nil {
		log.Fatalf("failed to create Spanner client: %v", err)
	}

	return spannerClient
}

func TestMain(m *testing.M) {
	emulatorHost := os.Getenv("SPANNER_EMULATOR_HOST")
	if emulatorHost == "" {
		log.Println("WARNING: SPANNER_EMULATOR_HOST environment variable is not set. Tests requiring Spanner will be skipped.")
	}
	// Run tests
	exitCode := m.Run()

	// Teardown: Drop the test database after all tests in the package have run.
	// This is optional if the emulator is ephemeral or if you prefer to clean up manually.
	if emulatorHost != "" { // Only attempt cleanup if emulator was supposed to be running
		ctx := context.Background()
		dbPath := fmt.Sprintf("projects/%s/instances/%s/databases/%s", testProjectID, testInstanceID, testDatabaseID)
		dbAdminClient, err := database.NewDatabaseAdminClient(ctx)
		if err == nil {
			defer dbAdminClient.Close()
			log.Printf("Dropping test database: %s", dbPath)
			err = dbAdminClient.DropDatabase(ctx, &adminpb.DropDatabaseRequest{Database: dbPath})
			if err != nil {
				log.Printf("Failed to drop test database %s: %v", dbPath, err)
			} else {
				log.Printf("Successfully dropped test database: %s", dbPath)
			}
		} else {
			log.Printf("Failed to create DatabaseAdminClient for teardown: %v", err)
		}
	}

	os.Exit(exitCode)
}

func TestSingersHandler_RandomInsert(t *testing.T) {
	spannerClient := setupTestDB(t)
	if spannerClient == nil { // Indicates SPANNER_EMULATOR_HOST was not set
		return
	}
	defer spannerClient.Close()

	store, err := NewSingersStore(context.Background(), spannerClient)
	if err != nil {
		t.Fatalf("Failed to create SingersStore: %v", err)
	}
	handler := NewSingersHandler(store)

	t.Run("valid request", func(t *testing.T) {
		// Clean slate for each sub-test if necessary, or ensure data from one doesn't affect another.
		// For this test, we are inserting, so previous inserts might affect count if not handled.
		// A simple way is to delete all singers before each test run or use unique IDs.
		// For simplicity, we'll rely on the fact that RandomInsert creates new UUIDs.

		count := 5
		body := randomInsertRequest{Count: count}
		jsonBody, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, "/singers/random-insert", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.RandomInsert(rr, req)

		if status := rr.Code; status != http.StatusCreated {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
			t.Errorf("response body: %s", rr.Body.String())
		}

		// Verify data in Spanner
		ctx := context.Background()
		iter := spannerClient.Single().Read(ctx, SingersTableName, spanner.AllKeys(), []string{SingersPK})
		defer iter.Stop()

		rowCount := 0
		for {
			_, err := iter.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatalf("Failed to iterate rows: %v", err)
			}
			rowCount++
		}
		// This check is tricky because BatchInsert in SingersStore generates new UUIDs for SingerID
		// and doesn't return them. The test currently inserts `count` singers,
		// but the handler also generates UUIDs for SingerID, FirstName, LastName.
		// The store's BatchInsert *also* generates new UUIDs for SingerID.
		// So, the number of singers in the DB should be `count`.
		// We need to ensure the test setup correctly clears the table or counts accurately.
		// Let's assume setupTestDB provides a clean DB or we clear it here.
		// Verify data in Spanner
		// setupTestDB ensures a clean database, so we can directly check the count after one insertion.
		// ctx was already defined above in this scope.
		iterAfterInsert := spannerClient.Single().Read(ctx, SingersTableName, spanner.AllKeys(), []string{SingersPK, "FirstName", "LastName"})
		defer iterAfterInsert.Stop()
		insertedCount := 0
		for {
			row, err := iterAfterInsert.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				t.Fatalf("Failed to iterate rows after insert: %v", err)
			}
			insertedCount++
			var singer Singer
			if err := row.ToStruct(&singer); err != nil {
				t.Fatalf("Failed to convert row to singer: %v", err)
			}
			if singer.SingerID == "" {
				t.Errorf("Expected SingerID to be non-empty")
			}
			if singer.FirstName == "" {
				t.Errorf("Expected FirstName to be non-empty")
			}
			if singer.LastName == "" {
				t.Errorf("Expected LastName to be non-empty")
			}
		}

		if insertedCount != count {
			t.Errorf("Expected %d singers to be inserted, but found %d", count, insertedCount)
		}
	})

	t.Run("invalid request body - count less than 1", func(t *testing.T) {
		// Clear table before this test to ensure clean state for other potential verifications
		_, err := spannerClient.Apply(context.Background(), []*spanner.Mutation{spanner.Delete(SingersTableName, spanner.AllKeys())})
		if err != nil {
			t.Fatalf("Failed to clear Singers table: %v", err)
		}

		body := randomInsertRequest{Count: 0}
		jsonBody, _ := json.Marshal(body)
		req, err := http.NewRequest(http.MethodPost, "/singers/random-insert", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.RandomInsert(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}

		expectedErrorMsg := "count must be greater than 0"
		if !strings.Contains(rr.Body.String(), expectedErrorMsg) {
			t.Errorf("handler returned unexpected body: got %v want substring %q", rr.Body.String(), expectedErrorMsg)
		}
	})

	t.Run("invalid request body - malformed json", func(t *testing.T) {
		// Clear table
		_, err := spannerClient.Apply(context.Background(), []*spanner.Mutation{spanner.Delete(SingersTableName, spanner.AllKeys())})
		if err != nil {
			t.Fatalf("Failed to clear Singers table: %v", err)
		}

		jsonBody := []byte(`{"count": "not-a-number"`) // Malformed JSON
		req, err := http.NewRequest(http.MethodPost, "/singers/random-insert", bytes.NewBuffer(jsonBody))
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.RandomInsert(rr, req)

		if status := rr.Code; status != http.StatusBadRequest {
			t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
		}
		expectedErrorMsg := "bad request"
		if !strings.Contains(rr.Body.String(), expectedErrorMsg) {
			t.Errorf("handler returned unexpected body for malformed json: got %v want substring %q", rr.Body.String(), expectedErrorMsg)
		}
	})

	t.Run("invalid http method", func(t *testing.T) {
		// Clear table
		_, err := spannerClient.Apply(context.Background(), []*spanner.Mutation{spanner.Delete(SingersTableName, spanner.AllKeys())})
		if err != nil {
			t.Fatalf("Failed to clear Singers table: %v", err)
		}

		req, err := http.NewRequest(http.MethodGet, "/singers/random-insert", nil) // Using GET
		if err != nil {
			t.Fatalf("Failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.RandomInsert(rr, req)

		if status := rr.Code; status != http.StatusMethodNotAllowed {
			t.Errorf("handler returned wrong status code for GET: got %v want %v", status, http.StatusMethodNotAllowed)
		}
		expectedErrorMsg := "method not allowed"
		if !strings.Contains(rr.Body.String(), expectedErrorMsg) {
			t.Errorf("handler returned unexpected body for GET: got %v want substring %q", rr.Body.String(), expectedErrorMsg)
		}
	})
}

// Helper function to count singers in the database
func countSingers(ctx context.Context, client *spanner.Client) (int, error) {
	iter := client.Single().Read(ctx, SingersTableName, spanner.AllKeys(), []string{SingersPK})
	defer iter.Stop()
	count := 0
	for {
		_, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("failed to iterate rows: %w", err)
		}
		count++
	}
	return count, nil
}
