package hopper

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"github.com/sinmetal/hopper/internal/trace"
	"google.golang.org/api/iterator"
)

const (
	SingersTableName = "Singers"
	SingersPK        = "SingerID"
)

// Singer is a model for Singers table
type Singer struct {
	SingerID  string    `spanner:"SingerID"`
	FirstName string    `spanner:"FirstName"`
	LastName  string    `spanner:"LastName"`
	CreatedAt time.Time `spanner:"CreatedAt"`
	UpdatedAt time.Time `spanner:"UpdatedAt"`
}

// SingersStore is a store for Singers table.
type SingersStore struct {
	sc *spanner.Client
}

// NewSingersStore returns a new SingersStore.
func NewSingersStore(ctx context.Context, sc *spanner.Client) (*SingersStore, error) {
	return &SingersStore{
		sc: sc,
	}, nil
}

// BatchInsert inserts new singers.
func (s *SingersStore) BatchInsert(ctx context.Context, singers []*Singer) error {
	ctx, span := trace.StartSpan(ctx, "SingersStore.BatchInsert")
	defer span.End()

	var ms []*spanner.Mutation
	for _, singer := range singers {
		singer.SingerID = uuid.New().String()
		ms = append(ms, spanner.InsertMap(SingersTableName, map[string]interface{}{
			"SingerID":  singer.SingerID,
			"FirstName": singer.FirstName,
			"LastName":  singer.LastName,
			"CreatedAt": spanner.CommitTimestamp,
			"UpdatedAt": spanner.CommitTimestamp,
		}))
	}
	_, err := s.sc.Apply(ctx, ms)
	if err != nil {
		return fmt.Errorf("failed to apply mutation: %w", err)
	}
	return nil
}

// Get returns a singer by primary key.
func (s *SingersStore) Get(ctx context.Context, id string) (*Singer, error) {
	ctx, span := trace.StartSpan(ctx, "SingersStore.Get")
	defer span.End()

	row, err := s.sc.Single().ReadRow(ctx, SingersTableName, spanner.Key{id}, []string{"SingerID", "FirstName", "LastName", "CreatedAt", "UpdatedAt"})
	if err != nil {
		return nil, fmt.Errorf("failed to read row: %w", err)
	}
	var singer Singer
	if err := row.ToStruct(&singer); err != nil {
		return nil, fmt.Errorf("failed to convert row to struct: %w", err)
	}
	return &singer, nil
}

// List returns all singers.
func (s *SingersStore) List(ctx context.Context) ([]*Singer, error) {
	ctx, span := trace.StartSpan(ctx, "SingersStore.List")
	defer span.End()

	iter := s.sc.Single().Read(ctx, SingersTableName, spanner.AllKeys(), []string{"SingerID", "FirstName", "LastName", "CreatedAt", "UpdatedAt"})
	defer iter.Stop()

	var singers []*Singer
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get next row: %w", err)
		}
		var singer Singer
		if err := row.ToStruct(&singer); err != nil {
			return nil, fmt.Errorf("failed to convert row to struct: %w", err)
		}
		singers = append(singers, &singer)
	}
	return singers, nil
}

// ListByCreatedAt returns singers created before the specified days.
func (s *SingersStore) ListByCreatedAt(ctx context.Context, oldDay int, limit int) ([]*Singer, error) {
	ctx, span := trace.StartSpan(ctx, "SingersStore.ListByCreatedAt")
	defer span.End()

	stmt := spanner.NewStatement(`
		SELECT
			SingerID,
			FirstName,
			LastName,
			CreatedAt,
			UpdatedAt
		FROM
			Singers
		WHERE
			CreatedAt < @createdAt
		ORDER BY CreatedAt DESC
		LIMIT @limit
	`)
	var filterCreatedAt = time.Now()
	if oldDay != 0 {
		filterCreatedAt = filterCreatedAt.AddDate(0, 0, -oldDay)
	}
	stmt.Params["createdAt"] = filterCreatedAt
	stmt.Params["limit"] = limit

	iter := s.sc.Single().Query(ctx, stmt)
	defer iter.Stop()

	var singers []*Singer
	for {
		row, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get next row: %w", err)
		}
		var singer Singer
		if err := row.ToStruct(&singer); err != nil {
			return nil, fmt.Errorf("failed to convert row to struct: %w", err)
		}
		singers = append(singers, &singer)
	}
	return singers, nil
}

// Update updates a singer.
func (s *SingersStore) Update(ctx context.Context, singer *Singer) error {
	ctx, span := trace.StartSpan(ctx, "SingersStore.Update")
	defer span.End()

	m := spanner.UpdateMap(SingersTableName, map[string]interface{}{
		"SingerID":  singer.SingerID,
		"FirstName": singer.FirstName,
		"LastName":  singer.LastName,
		"UpdatedAt": spanner.CommitTimestamp,
	})
	_, err := s.sc.Apply(ctx, []*spanner.Mutation{m})
	if err != nil {
		return fmt.Errorf("failed to apply mutation: %w", err)
	}
	return nil
}
