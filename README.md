## Sprint 0 — Decisions

- [ ] MVP scope: HTTP GET only (latency, status/timeout)
- [ ] Data model fixed:
    - [ ] Domain(id, name, selected_period)
    - [ ] Check(id, domain_id, scheme, method=GET, path, timeout_ms, frequency, realtime)
    - [ ] Result(id, check_id, ts, duration_ms, status_code?, outcome)
- [ ] Storage: SQLite via database/sql (driver выбрать на Sprint 1)
- [ ] Scheduling: time.Ticker for periodic checks; realtime loop for continuous
- [ ] Cancellation: context for all workers/requests
- [ ] Rate limiting: global + per-domain using token bucket
- [ ] API shape planned: REST (CRUD + results), SSE for realtime points
