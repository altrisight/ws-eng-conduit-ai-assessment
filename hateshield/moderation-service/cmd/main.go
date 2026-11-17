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

// When set, /v1/* endpoints require "Authorization: Bearer <token>".
var authToken = os.Getenv("MODERATION_TOKEN")

// JSONL file persisting queue state across restarts; empty = memory only.
var queuePath = os.Getenv("QUEUE_PATH")

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

func persist(it *QueueItem) error {
	if queuePath == "" {
		return nil
	}
	f, err := os.OpenFile(queuePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(it); err != nil {
		return err
	}
	return f.Sync()
}

// loadQueue replays the JSONL log; later records for a post_id win.
func loadQueue() {
	if queuePath == "" {
		return
	}
	data, err := os.ReadFile(queuePath)
	if err != nil {
		return
	}
	lines := bytes.Split(data, []byte("\n"))
	// A truncated final line (partial append) would join the next write to it;
	// cut the file back to the last complete line.
	if len(data) > 0 && data[len(data)-1] != '\n' {
		lastNL := bytes.LastIndex(data, []byte("\n"))
		os.Truncate(queuePath, int64(lastNL+1))
		lines = lines[:len(lines)-1]
	}
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var it QueueItem
		if json.Unmarshal(line, &it) == nil {
			queue[it.PostID] = &it
		}
	}
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

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
	if err := persist(it); err != nil {
		mu.Unlock()
		http.Error(w, "queue storage unavailable", http.StatusInternalServerError)
		return
	}
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
		if err := persist(it); err != nil {
			mu.Unlock()
			http.Error(w, "queue storage unavailable", http.StatusInternalServerError)
			return
		}
	}
	mu.Unlock()

	// Record the verdict as a training label in feedback-service.
	label, _ := json.Marshal(map[string]string{
		"post_id": fb.PostID,
		"label":   fb.Label,
		"source":  "moderator",
	})
	req, _ := http.NewRequest("POST", feedbackURL+"/v1/labels", bytes.NewReader(label))
	req.Header.Set("Content-Type", "application/json")
	if t := os.Getenv("FEEDBACK_TOKEN"); t != "" {
		req.Header.Set("Authorization", "Bearer "+t)
	}
	go http.DefaultClient.Do(req)

	w.WriteHeader(http.StatusAccepted)
}

func main() {
	loadQueue()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /v1/queue", requireAuth(listQueue))
	mux.HandleFunc("POST /v1/queue", requireAuth(enqueue))
	mux.HandleFunc("POST /v1/feedback", requireAuth(submitFeedback))

	log.Println("moderation-service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", mux))
}
