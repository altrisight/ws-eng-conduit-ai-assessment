package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var feedbackURL = env("FEEDBACK_URL", "http://localhost:8083")

type QueueItem struct {
	PostID  string             `json:"post_id"`
	Scores  map[string]float64 `json:"scores"`
	Status  string             `json:"status"` // pending | approved | rejected
}

type Feedback struct {
	PostID string `json:"post_id"`
	Label  string `json:"label"` // moderator verdict
}

var (
	mu    sync.Mutex
	queue = map[string]*QueueItem{}
)

func listQueue(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	items := make([]*QueueItem, 0, len(queue))
	for _, it := range queue {
		if it.Status == "pending" {
			items = append(items, it)
		}
	}
	json.NewEncoder(w).Encode(items)
}

func enqueue(w http.ResponseWriter, r *http.Request) {
	var it QueueItem
	if err := json.NewDecoder(r.Body).Decode(&it); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	mu.Lock()
	it.Status = "pending"
	queue[it.PostID] = &it
	mu.Unlock()
	w.WriteHeader(http.StatusCreated)
}

func submitFeedback(w http.ResponseWriter, r *http.Request) {
	var fb Feedback
	if err := json.NewDecoder(r.Body).Decode(&fb); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	mu.Lock()
	if it, ok := queue[fb.PostID]; ok {
		if fb.Label == "hate" {
			it.Status = "approved"
		} else {
			it.Status = "rejected"
		}
	}
	mu.Unlock()

	// Record the verdict as a training label in feedback-service.
	label, _ := json.Marshal(map[string]string{
		"post_id": fb.PostID,
		"label":   fb.Label,
		"source":  "moderator",
	})
	go http.Post(feedbackURL+"/v1/labels", "application/json", bytes.NewReader(label))

	w.WriteHeader(http.StatusAccepted)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /v1/queue", listQueue)
	mux.HandleFunc("POST /v1/queue", enqueue)
	mux.HandleFunc("POST /v1/feedback", submitFeedback)

	log.Println("moderation-service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", mux))
}
