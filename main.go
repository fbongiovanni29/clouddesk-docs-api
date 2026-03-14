package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Document represents a stored document.
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var (
	store = make(map[string]Document)
	mu    sync.RWMutex
)

func main() {
	http.HandleFunc("/healthz", handleHealthz)
	http.HandleFunc("/docs", handleDocs)
	http.HandleFunc("/docs/", handleDocByID)
	log.Println("docs-api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// handleHealthz returns 200 OK for liveness/readiness probes.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func handleDocs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		docs := make([]Document, 0, len(store))
		for _, d := range store {
			docs = append(docs, d)
		}
		mu.RUnlock()
		writeJSON(w, http.StatusOK, docs)

	case http.MethodPost:
		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		now := time.Now().UTC()
		doc := Document{
			ID:        uuid.New().String(),
			Title:     input.Title,
			Content:   input.Content,
			CreatedAt: now,
			UpdatedAt: now,
		}
		mu.Lock()
		store[doc.ID] = doc
		mu.Unlock()
		log.Printf("POST /docs created id=%s title=%q", doc.ID, doc.Title)
		writeJSON(w, http.StatusCreated, doc)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleDocByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/docs/"):]
	if id == "" {
		http.Error(w, "missing document id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		doc, ok := store[id]
		mu.RUnlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, doc)

	case http.MethodPut:
		if r.Body == nil {
			http.Error(w, "missing request body", http.StatusBadRequest)
			return
		}
		var input struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		mu.Lock()
		doc, ok := store[id]
		if !ok {
			mu.Unlock()
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		doc.Title = input.Title
		doc.Content = input.Content
		doc.UpdatedAt = time.Now().UTC()
		store[id] = doc
		mu.Unlock()
		log.Printf("PUT /docs/%s updated title=%q", id, doc.Title)
		writeJSON(w, http.StatusOK, doc)

	case http.MethodDelete:
		mu.Lock()
		_, ok := store[id]
		if ok {
			delete(store, id)
		}
		mu.Unlock()
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("DELETE /docs/%s", id)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
