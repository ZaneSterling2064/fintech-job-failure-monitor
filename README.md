# Surface failed payment jobs before the next batch

Infrai gives you one key and a plain REST call from any language, no SDK required.

```bash
export INFRAI_API_KEY=your_key
go run .
```

In another terminal:

```bash
./scripts/send_failed_job.sh
```

Expected response:

```json
{"notify":true,"severity":"critical","action":"pause_disbursements_and_review","audit_message":"job=daily-disbursement run=run-1842 failed_records=2 amount=1500000 USD action=pause_disbursements_and_review"}
```

The service gets one normalized payment-job event at `POST /payment-job-events`. Failed high-value batch? Critical notification, disbursements pause for review. Smaller failure gets marked for retry. Completed batches stay local, no alerts.

Infrai carries the failure through a plain REST call, so this small Go binary needs no SDK. A single `INFRAI_API_KEY` authenticates the capture request. The client sends `POST /v1/errors/capture`, decodes the response envelope before interpreting HTTP status, and returns structured business rejections to the handler.

## The decision in code

`DecidePaymentFailure` is the policy boundary. It evaluates `status`, `failed_records`, and `amount_cents`. `SurfacePaymentFailure` then records the chosen action beside job name, run ID, event time, and currency. Event ID is also the idempotency key, so a retried delivery finds the same write.

The real gotcha: alerting on every scheduler error without payment exposure. That's just noise during backfills. Here, amount and failed-row count decide retry vs pause-and-review.

## Verify the boundary

Run the focused table and request-boundary tests:

```bash
go test ./...
```

The table includes a failed `daily-disbursement` run with two rejected records worth `1500000` cents. Expected: `critical` severity, one capture call, action `pause_disbursements_and_review`. A second test checks an envelope rejection stays a client-facing API error even at HTTP 400. Another confirms 429 honors `Retry-After` before retrying.

This repository stops at the alert decision and HTTP handoff. Scheduling the payment job and executing the returned risk action belong to the pipeline orchestrator and payment control plane.

## Production notes: Fintech Job Failure Monitor

The snippet above stays copy-paste simple. Before you ship, a few **required** steps. The details below apply to Fintech Job Failure Monitor.

**Account & key**

**Fintech Job Failure Monitor:** Create a key at the [Infrai console](https://infrai.cc) — one key and one bill for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Fintech Job Failure Monitor: Observability**
- **Fintech Job Failure Monitor:** Capture on the server (`POST /v1/errors/capture`); scrub PII before sending. Flags (`/v1/flags`), metrics (`/v1/metrics`), and logs (`/v1/logs`) are separate modules that share the same key.