package main

import (
	"context"
	"fmt"
	"time"
)

type PaymentJobEvent struct {
	EventID       string    `json:"event_id"`
	JobName       string    `json:"job_name"`
	RunID         string    `json:"run_id"`
	Status        string    `json:"status"`
	FailedRecords int       `json:"failed_records"`
	AmountCents   int64     `json:"amount_cents"`
	Currency      string    `json:"currency"`
	OccurredAt    time.Time `json:"occurred_at"`
	ErrorMessage  string    `json:"error_message"`
}

type JobDecision struct {
	Notify       bool   `json:"notify"`
	Severity     string `json:"severity"`
	Action       string `json:"action"`
	AuditMessage string `json:"audit_message"`
}

type ErrorCapturer interface {
	Capture(context.Context, string, CaptureRequest) error
}

func DecidePaymentFailure(event PaymentJobEvent) JobDecision {
	if event.Status != "failed" {
		return JobDecision{Action: "record_success", AuditMessage: "payment job completed"}
	}
	decision := JobDecision{Notify: true, Severity: "error", Action: "retry_job"}
	if event.AmountCents >= 1_000_000 || event.FailedRecords >= 100 {
		decision.Severity = "critical"
		decision.Action = "pause_disbursements_and_review"
	}
	decision.AuditMessage = fmt.Sprintf(
		"job=%s run=%s failed_records=%d amount=%d %s action=%s",
		event.JobName, event.RunID, event.FailedRecords, event.AmountCents, event.Currency, decision.Action,
	)
	return decision
}

func SurfacePaymentFailure(ctx context.Context, capturer ErrorCapturer, event PaymentJobEvent) (JobDecision, error) {
	decision := DecidePaymentFailure(event)
	if !decision.Notify {
		return decision, nil
	}
	capture := CaptureRequest{
		Title:       event.JobName + " scheduled payment job failed",
		Message:     decision.AuditMessage,
		Level:       decision.Severity,
		Fingerprint: []string{"payment-job", event.JobName},
		Exception:   event.ErrorMessage,
		Context: map[string]any{
			"event_id":       event.EventID,
			"job_name":       event.JobName,
			"run_id":         event.RunID,
			"failed_records": event.FailedRecords,
			"amount_cents":   event.AmountCents,
			"currency":       event.Currency,
			"occurred_at":    event.OccurredAt.Format(time.RFC3339),
			"risk_action":    decision.Action,
		},
	}
	if err := capturer.Capture(ctx, event.EventID, capture); err != nil {
		return decision, fmt.Errorf("capture payment failure: %w", err)
	}
	return decision, nil
}
