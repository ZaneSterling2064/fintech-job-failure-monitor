#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/payment-job-events \
  --header 'Content-Type: application/json' \
  --data '{"event_id":"payevt-20260913-01","job_name":"daily-disbursement","run_id":"run-1842","status":"failed","failed_records":2,"amount_cents":1500000,"currency":"USD","occurred_at":"2026-09-13T01:30:00Z","error_message":"bank response rejected"}'
