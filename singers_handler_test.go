package hopper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"cloud.google.com/go/spanner"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	instancepb "google.golang.org/genproto/googleapis/spanner/admin/instance/v1"
	databasepb "google.golang.org/genproto/googleapis/spanner/admin/database/v1"
)

const (
	projectID = "test-project"
	instanceID = "test-instance"
	databaseID = "test-database"
)

func TestMain(m *testing.M) {
	if os.Getenv("SPANNER_EMULATOR_HOST") == "" {
		fmt.Println("SPANNER_EMULATOR_HOST not set")
		os.Exit(0)
	}

	ctx := context.Background()
	if err := setup(ctx); err != nil {
		fmt.Printf("failed to setup: %v", err)
		os.Exit(1)
	}

	code := m.Run()

	if err := teardown(ctx); err != nil {
		fmt.Printf("failed to teardown: %v", err)
		os.Exit(1)
	}

	os.Exit(code)
}

func setup(ctx context.Context) error {
	conn, err := grpc.Dial(os.Getenv("SPANNER_EMULATOR_HOST"), grpc.WithInsecure())
	if err != nil {
		return err
	}

	instanceAdminClient, err := instance.NewInstanceAdminClient(ctx, option.WithGRPCConn(conn))
	if err != nil {
		return err
	}
	defer instanceAdminClient.Close()

	databaseAdminClient, err := database.NewDatabaseAdminClient(ctx, option.WithGRPCConn(conn))
	if err != nil {
		return err
	}
	defer databaseAdminClient.Close()

	_, err = instanceAdminClient.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     fmt.Sprintf("projects/%s", projectID),
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			Config:      fmt.Sprintf("projects/%s/instanceConfigs/emulator-config", projectID),
			DisplayName: "Test Instance",
			NodeCount:   1,
		},
	})
	if err != nil {
		return err
	}

	op, err := databaseAdminClient.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          fmt.Sprintf("projects/%s/instances/%s", projectID, instanceID),
		CreateStatement: "CREATE DATABASE `" + databaseID + "`",
		ExtraStatements: []string{
			`CREATE TABLE Singers (
				SingerID STRING(MAX) NOT NULL,
				FirstName STRING(1024),
				LastName STRING(1024),
				CreatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE),
				UpdatedAt TIMESTAMP NOT NULL OPTIONS (allow_commit_timestamp=TRUE)
			) PRIMARY KEY (SingerID)`,
		},
	})
	if err != nil {
		return err
	}
	if _, err := op.Wait(ctx); err != nil {
		return err
	}

	return nil
}

func teardown(ctx context.Context) error {
	conn, err := grpc.Dial(os.Getenv("SPANNER_EMULATOR_HOST"), grpc.WithInsecure())
	if err != nil {
		return err
	}

	databaseAdminClient, err := database.NewDatabaseAdminClient(ctx, option.WithGRPCConn(conn))
	if err != nil {
		return err
	}
	defer databaseAdminClient.Close()

	if err := databaseAdminClient.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectID, instanceID, databaseID),
	}); err != nil {
		return err
	}

	return nil
}

func TestSingersHandler_RandomInsert(t *testing.T) {
	ctx := context.Background()
	dsn := fmt.Sprintf("projects/%s/instances/%s/databases/%s", projectID, instanceID, databaseID)

	conn, err := grpc.Dial(os.Getenv("SPANNER_EMULATOR_HOST"), grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}
	client, err := spanner.NewClient(ctx, dsn, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	store, err := NewSingersStore(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	h := NewSingersHandler(store)

	body := randomInsertRequest{
		Count: 10,
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/singers/random-insert", bytes.NewReader(b))
	rw := httptest.NewRecorder()

	h.RandomInsert(rw, req)

	if rw.Code != http.StatusCreated {
		t.Errorf("unexpected status code: got %d, want %d", rw.Code, http.StatusCreated)
	}

	singers, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(singers) != 10 {
		t.Errorf("unexpected singers count: got %d, want %d", len(singers), 10)
	}
}