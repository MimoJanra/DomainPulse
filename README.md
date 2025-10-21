# DomainPulse

> Lightweight uptime and performance monitoring tool for domains and endpoints.  
> Collect response times and HTTP status codes, visualize uptime trends, and receive real-time updates.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)
![Status](https://img.shields.io/badge/status-Active-blue)

---

## 🚀 Overview

**DomainPulse** performs periodic or real-time checks of HTTP endpoints  
(with planned support for TCP, UDP, and ICMP).  
It records response times, classifies results (timeouts, 2xx, 4xx, 5xx),  
and visualizes them on interactive charts.

### Key Features

- HTTP monitoring (GET / POST / PUT) with custom paths and payloads
- Adjustable check intervals: **1s / 1m / 1h / 1d**
- Real-time mode: new request starts immediately after the previous response
- Color-coded result points:
    - 🟢 **2xx** — OK
    - 🟡 **4xx** — Client error
    - 🔴 **5xx** — Server error
    - ⚫ **Timeout / no response**
- SQLite storage (simple and portable)
- REST API and SSE stream for live updates
- Extensible for additional check types (TCP / UDP / ICMP)

---

## 🖥️ User Interface

### Dashboard
- Displays monitored domains and their current status
- “➕” button — add a new domain check
- Columns: domain, check type, frequency, last response

### Domain View
- Summary metrics: uptime %, p50/p95 latency
- Compact timeline chart with color-coded points
- Click to expand into a full chart view

### Chart View
- Zoom, pan, and select custom time ranges
- Selected range reflects back on the main dashboard

### Settings (⚙️)
- Edit check parameters: frequency, timeout, method, path
- Delete or disable checks

---

## 🧩 Data Model

| Entity | Fields |
|:--------|:--------|
| **Domain** | id, name, selected_period |
| **Check** | id, domain_id, scheme, method, path, timeout_ms, frequency, realtime |
| **Result** | id, check_id, timestamp, duration_ms, status_code, outcome (`timeout`, `2xx`, `4xx`, `5xx`) |

---

## 🔌 API Endpoints

| Method | Path | Description |
|:--------|:------|:-------------|
| `POST` | `/domains` | Create domain |
| `GET` | `/domains` | List domains |
| `GET` | `/domains/{id}` | Get domain summary |
| `POST` | `/checks` | Create check |
| `PATCH` | `/checks/{id}` | Update check parameters |
| `DELETE` | `/checks/{id}` | Remove check |
| `GET` | `/results?check_id=&from=&to=` | Fetch results |
| `GET` | `/stream/checks/{id}` | Stream results via SSE |

---

## ⚙️ Configuration

| Setting | Description |
|:----------|:-------------|
| `frequency` | Interval between checks (`1s`, `1m`, `1h`, `1d`) |
| `realtime` | Run next check immediately after previous completes |
| `timeout_ms` | Timeout for each request |
| `scheme` | `http` or `https` |
| `method` | HTTP method: `GET`, `POST`, `PUT` |
| `path` | Path after domain (e.g. `/health`) |
| `content` | Optional body for `POST` / `PUT` |

---

## 🛡️ Reliability & Rate Limiting

- Each check runs in its own worker using `time.Ticker`
- Graceful cancellation via `context`
- Global and per-domain rate limiting with token buckets
- Strict HTTP client timeouts
- Built-in protection from over-polling (anti-DoS behavior)

---

## 🧠 Technical References

| Topic | Documentation |
|:--------|:---------------|
| HTTP client & timeouts | [`net/http`](https://pkg.go.dev/net/http) |
| Context & cancellation | [`context`](https://pkg.go.dev/context) |
| Scheduling | [`time.Ticker`](https://pkg.go.dev/time) |
| Database layer | [`database/sql`](https://pkg.go.dev/database/sql) |
| SQLite driver | [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) |
| Rate limiting | [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) |
| Server-Sent Events (SSE) | [MDN — Server-Sent Events](https://developer.mozilla.org/docs/Web/API/Server-sent_events) |
| HTTP status codes | [MDN — HTTP Status](https://developer.mozilla.org/docs/Web/HTTP/Status) |
