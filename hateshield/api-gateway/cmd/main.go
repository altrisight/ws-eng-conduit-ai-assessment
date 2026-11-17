package main

import (
	"io"
	"log"
	"net/http"
	"os"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var ingestionURL = env("INGESTION_URL", "http://localhost:8081")
var moderationURL = env("MODERATION_URL", "http://localhost:8082")

// proxy forwards the request to a backend service and copies the response.
func proxy(backend string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest(r.Method, backend+r.URL.Path, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		req.Header = r.Header.Clone()
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/v1/analyze", proxy(ingestionURL))
	mux.HandleFunc("/v1/queue", proxy(moderationURL))
	mux.HandleFunc("/v1/queue/", proxy(moderationURL))
	mux.HandleFunc("/v1/feedback", proxy(moderationURL))

	log.Println("api-gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
