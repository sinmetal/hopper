package hopper

import (
	"encoding/json"
	"log"
	"math/rand"
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
	ctx := r.Context()

	if r.Method != http.MethodPost {
		log.Printf("method not allowed. got %s", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body randomInsertRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("failed to decode request body. %s", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if body.Count < 1 {
		log.Printf("count must be greater than 0. got %d", body.Count)
		http.Error(w, "count must be greater than 0", http.StatusBadRequest)
		return
	}

	singers := make([]*Singer, body.Count)
	for i := 0; i < body.Count; i++ {
		u, err := uuid.NewRandom()
		if err != nil {
			log.Printf("failed to generate uuid. %s", err)
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
		log.Printf("failed to insert singers. %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

type randomUpdateRequest struct {
	OldDay int `json:"oldDay"`
}

// RandomUpdate is POST /singers/random-update
func (h *SingersHandler) RandomUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		log.Printf("method not allowed. got %s", r.Method)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body randomUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		log.Printf("failed to decode request body. %s", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if body.OldDay < 1 {
		log.Printf("oldDay must be greater than 0. got %d", body.OldDay)
		http.Error(w, "oldDay must be greater than 0", http.StatusBadRequest)
		return
	}

	singers, err := h.Store.ListByCreatedAt(ctx, body.OldDay, 10)
	if err != nil {
		log.Printf("failed to list singers. %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if len(singers) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	target := singers[rand.Intn(len(singers))]
	target.FirstName = uuid.New().String()
	target.LastName = uuid.New().String()

	if err := h.Store.Update(ctx, target); err != nil {
		log.Printf("failed to update singer. %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
