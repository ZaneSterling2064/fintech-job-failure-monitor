package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCaptureDecodesRejectionBeforeHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", request.Method)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"ok":false,"data":null,"error":{"code":"rejected","message":"event rejected"},"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient("test-key")
	client.baseURL = server.URL
	err := client.Capture(context.Background(), "evt-1", CaptureRequest{Exception: "row rejected"})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "rejected" || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %#v, want decoded APIError", err)
	}
}

func TestCaptureRetriesRateLimit(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		writer.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			writer.Header().Set("Retry-After", "2")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = writer.Write([]byte(`{"ok":false,"error":{"code":"rate_limited","message":"retry later"},"metadata":{}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"ok":true,"data":{"event_id":"accepted"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := NewInfraiClient("test-key")
	client.baseURL = server.URL
	var slept time.Duration
	client.sleep = func(_ context.Context, delay time.Duration) error { slept = delay; return nil }
	if err := client.Capture(context.Background(), "evt-2", CaptureRequest{Exception: "batch rejected"}); err != nil {
		t.Fatalf("Capture() error = %v", err)
	}
	if calls != 2 || slept != 2*time.Second {
		t.Fatalf("calls=%d slept=%s, want calls=2 slept=2s", calls, slept)
	}
}
