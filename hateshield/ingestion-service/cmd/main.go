package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var mlURL = env("ML_INFERENCE_URL", "http://localhost:9000")
var moderationURL = env("MODERATION_URL", "http://localhost:8082")

type AnalyzeRequest struct {
	ContentType string `json:"content_type"` // text | audio | video
	Text        string `json:"text,omitempty"`
	MediaURL    string `json:"media_url,omitempty"`
	PostID      string `json:"post_id,omitempty"`
}

type AnalyzeResponse struct {
	PostID    string             `json:"post_id"`
	Scores    map[string]float64 `json:"scores"`
	Flagged   bool               `json:"flagged"`
	Transcript string            `json:"transcript,omitempty"`
}

func analyze(w http.ResponseWriter, r *http.Request) {
	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ContentType == "" {
		http.Error(w, "content_type required", http.StatusBadRequest)
		return
	}

	body, _ := json.Marshal(req)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Post(mlURL+"/analyze", "application/json", bytes.NewReader(body))
	if err != nil {
		http.Error(w, "ml inference unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed reading ml response", http.StatusBadGateway)
		return
	}

	if resp.StatusCode == http.StatusOK {
		var result AnalyzeResponse
		if json.Unmarshal(respBody, &result) == nil && result.Flagged {
			postID := result.PostID
			if postID == "" {
				postID = fmt.Sprintf("post-%d", time.Now().UnixNano())
			}
			item, _ := json.Marshal(map[string]any{
				"post_id": postID,
				"scores":  result.Scores,
			})
			go http.Post(moderationURL+"/v1/queue", "application/json", bytes.NewReader(item))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/analyze", analyze)

	log.Println("ingestion-service listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}
