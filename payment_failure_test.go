package main

import (
	"context"
	"testing"
	"time"
)

type recordingCapturer struct {
	calls   int
	key     string
	request CaptureRequest
}

func (r *recordingCapturer) Capture(_ context.Context, key string, request CaptureRequest) error {
	r.calls++
	r.key = key
	r.request = request
	return nil
}

func TestSurfacePaymentFailure(t *testing.T) {
	tests := []struct {
		name         string
		event        PaymentJobEvent
		wantNotify   bool
		wantSeverity string
		wantAction   string
		wantCalls    int
	}{
		{
			name:       "completed batch stays in the audit stream",
			event:      PaymentJobEvent{EventID: "evt-100", JobName: "settlement-import", RunID: "run-8", Status: "completed"},
			wantAction: "record_success",
		},
		{
			name:       "small failed batch is retried",
			event:      PaymentJobEvent{EventID: "evt-101", JobName: "settlement-import", RunID: "run-9", Status: "failed", FailedRecords: 4, AmountCents: 8500, Currency: "USD", ErrorMessage: "four rows failed validation"},
			wantNotify: true, wantSeverity: "error", wantAction: "retry_job", wantCalls: 1,
		},
		{
			name:       "high value failure pauses disbursements",
			event:      PaymentJobEvent{EventID: "evt-102", JobName: "daily-disbursement", RunID: "run-10", Status: "failed", FailedRecords: 2, AmountCents: 1_500_000, Currency: "USD", ErrorMessage: "bank response rejected"},
			wantNotify: true, wantSeverity: "critical", wantAction: "pause_disbursements_and_review", wantCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.event.OccurredAt = time.Date(2026, 9, 13, 1, 30, 0, 0, time.UTC)
			capturer := &recordingCapturer{}
			got, err := SurfacePaymentFailure(context.Background(), capturer, test.event)
			if err != nil {
				t.Fatalf("SurfacePaymentFailure() error = %v", err)
			}
			if got.Notify != test.wantNotify || got.Severity != test.wantSeverity || got.Action != test.wantAction {
				t.Fatalf("decision = %+v, want notify=%v severity=%q action=%q", got, test.wantNotify, test.wantSeverity, test.wantAction)
			}
			if capturer.calls != test.wantCalls {
				t.Fatalf("capture calls = %d, want %d", capturer.calls, test.wantCalls)
			}
			if test.wantCalls == 1 && capturer.key != test.event.EventID {
				t.Fatalf("idempotency key = %q, want %q", capturer.key, test.event.EventID)
			}
		})
	}
}
