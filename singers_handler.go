package hopper

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/google/uuid"
)

// SingersHandler is singers table handler
type SingersHandler struct {
	Store *SingersStore
}

// NewSingersHandler is SingersHandler constructor
func NewSingersHandler(store *SingersStore) *SingersHandler {
	return &SingersHandler{
		Store: store,
	}
}

type randomInsertRequest struct {
	Count int `json:"count"`
}

// RandomInsert is POST /singers/random-insert
func (h *SingersHandler) RandomInsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("method not allowed. got %s\n", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	var body randomInsertRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("failed to decode request body. %s\n", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if body.Count < 1 {
		log.Printf("count must be greater than 0. got %d\n", body.Count)
		http.Error(w, "count must be greater than 0", http.StatusBadRequest)
		return
	}

	singers := make([]*Singer, body.Count)
	for i := 0; i < body.Count; i++ {
		u, err := uuid.NewRandom()
		if err != nil {
			log.Printf("failed to generate uuid. %s\n", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		singers[i] = &Singer{
			SingerID:  u.String(),
			FirstName: uuid.New().String(),
			LastName:  uuid.New().String(),
		}
	}

	if err := h.Store.BatchInsert(ctx, singers); err != nil {
		log.Printf("failed to insert singers. %s\n", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
