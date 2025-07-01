package hopper

import (
	"context"
	"fmt"

	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

const (
	SingersTableName = "Singers"
	SingersPK        = "SingerID"
)

// Singer is a model for Singers table
type Singer struct {
	SingerID  string `spanner:"SingerID"`
	FirstName string `spanner:"FirstName"`
	LastName  string `spanner:"LastName"`
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

// Insert inserts a new singer.
func (s *SingersStore) Insert(ctx context.Context, singer *Singer) error {
	singer.SingerID = uuid.New().String()
	m, err := spanner.InsertStruct(SingersTableName, singer)
	if err != nil {
		return fmt.Errorf("failed to create insert struct: %w", err)
	}
	_, err = s.sc.Apply(ctx, []*spanner.Mutation{m})
	if err != nil {
		return fmt.Errorf("failed to apply mutation: %w", err)
	}
	return nil
}

// Get returns a singer by primary key.
func (s *SingersStore) Get(ctx context.Context, id string) (*Singer, error) {
	row, err := s.sc.Single().ReadRow(ctx, SingersTableName, spanner.Key{id}, []string{"SingerID", "FirstName", "LastName"})
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
	iter := s.sc.Single().Read(ctx, SingersTableName, spanner.AllKeys(), []string{"SingerID", "FirstName", "LastName"})
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
