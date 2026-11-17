package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
)

var labelsPath = os.Getenv("LABELS_PATH")

// When set, /v1/* endpoints require "Authorization: Bearer <token>".
var authToken = os.Getenv("FEEDBACK_TOKEN")

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

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
	if labelsPath != "" {
		f, err := os.OpenFile(labelsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Printf("persist label: %v", err)
		} else {
			json.NewEncoder(f).Encode(l)
			f.Close()
		}
	}
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
	mux.HandleFunc("POST /v1/labels", requireAuth(collect))
	mux.HandleFunc("GET /v1/labels/export", requireAuth(export))

	log.Println("feedback-service listening on :8083")
	log.Fatal(http.ListenAndServe(":8083", mux))
}
