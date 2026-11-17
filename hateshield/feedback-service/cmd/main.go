package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

type Label struct {
	PostID  string `json:"post_id"`
	Label   string `json:"label"`
	Source  string `json:"source"` // moderator | user_report
}

var (
	mu     sync.Mutex
	labels []Label
)

func collect(w http.ResponseWriter, r *http.Request) {
	var l Label
	if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	mu.Lock()
	labels = append(labels, l)
	mu.Unlock()
	w.WriteHeader(http.StatusAccepted)
}

func export(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	json.NewEncoder(w).Encode(labels)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /v1/labels", collect)
	mux.HandleFunc("GET /v1/labels/export", export)

	log.Println("feedback-service listening on :8083")
	log.Fatal(http.ListenAndServe(":8083", mux))
}
