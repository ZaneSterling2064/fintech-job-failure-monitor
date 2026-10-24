package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := NewInfraiClient(apiKey)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payment-job-events", func(writer http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
		var event PaymentJobEvent
		decoder := json.NewDecoder(request.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid payment event"})
			return
		}
		if event.EventID == "" || event.JobName == "" || event.RunID == "" || event.OccurredAt.IsZero() {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "event_id, job_name, run_id, and occurred_at are required"})
			return
		}
		decision, err := SurfacePaymentFailure(request.Context(), client, event)
		if err != nil {
			var apiErr *APIError
			if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				writeJSON(writer, apiErr.StatusCode, map[string]string{"error": apiErr.Error()})
				return
			}
			writeJSON(writer, http.StatusBadGateway, map[string]string{"error": "failure notification was not accepted"})
			return
		}
		writeJSON(writer, http.StatusAccepted, decision)
	})

	server := &http.Server{Addr: ":8080", Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("payment job monitor listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
