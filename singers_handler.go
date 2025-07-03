package hopper

import (
	"context"
	"encoding/json"
	"fmt"
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
		msg := fmt.Sprintf("method not allowed. got %s", r.Method)
		log.Println(msg)
		http.Error(w, msg, http.StatusMethodNotAllowed)
		return
	}

	ctx := context.Background()

	var body randomInsertRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		msg := fmt.Sprintf("failed to decode request body. %s", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if body.Count < 1 {
		msg := fmt.Sprintf("count must be greater than 0. got %d", body.Count)
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	singers := make([]*Singer, body.Count)
	for i := 0; i < body.Count; i++ {
		u, err := uuid.NewRandom()
		if err != nil {
			msg := fmt.Sprintf("failed to generate uuid. %s", err)
			log.Println(msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		singers[i] = &Singer{
			SingerID:  u.String(),
			FirstName: uuid.New().String(),
			LastName:  uuid.New().String(),
		}
	}

	if err := h.Store.BatchInsert(ctx, singers); err != nil {
		msg := fmt.Sprintf("failed to insert singers. %s", err)
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
