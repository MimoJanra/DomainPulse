package models

type Domain struct {
	ID   int    `json:"id" example:"1"`
	Name string `json:"name" example:"example.com"`
}

type CheckParams struct {
	Path      string            `json:"path,omitempty" example:"/health"`
	Port      int               `json:"port,omitempty" example:"80"`
	Payload   string            `json:"payload,omitempty" example:"ping"`
	TimeoutMS int               `json:"timeout_ms,omitempty" example:"5000"`
	Scheme    string            `json:"scheme,omitempty" example:"https"`
	Method    string            `json:"method,omitempty" example:"GET"`
	Body      string            `json:"body,omitempty" example:""`
	Headers   map[string]string `json:"headers,omitempty" example:"{\"Authorization\": \"Bearer token\", \"X-Custom-Header\": \"value\"}"`
}

type Check struct {
	ID                 int         `json:"id" example:"1"`
	DomainID           int         `json:"domain_id" example:"1"`
	Type               string      `json:"type" example:"http"`
	IntervalSeconds    int         `json:"interval_seconds" example:"60"`
	Params             CheckParams `json:"params"`
	Enabled            bool        `json:"enabled" example:"true"`
	Path               string      `json:"path,omitempty" example:"/"`
	RealtimeMode       bool        `json:"realtime_mode,omitempty" example:"false"`
	RateLimitPerMinute int         `json:"rate_limit_per_minute,omitempty" example:"60"`
}

type Result struct {
	ID           int    `json:"id" example:"1"`
	CheckID      int    `json:"check_id" example:"1"`
	Status       string `json:"status" example:"success"`
	StatusCode   int    `json:"status_code,omitempty" example:"200"`
	DurationMS   int    `json:"duration_ms" example:"150"`
	Outcome      string `json:"outcome,omitempty" example:"2xx"`
	ErrorMessage string `json:"error_message,omitempty" example:""`
	CreatedAt    string `json:"created_at" example:"2024-01-01T12:00:00Z"`
}

type ResultsResponse struct {
	Results    []Result `json:"results"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	TotalPages int      `json:"total_pages"`
}

type LatencyStats struct {
	Min    int     `json:"min" example:"50"`
	Max    int     `json:"max" example:"500"`
	Avg    float64 `json:"avg" example:"150.5"`
	Median float64 `json:"median" example:"145.0"`
	P95    float64 `json:"p95" example:"300.0"`
	P99    float64 `json:"p99" example:"450.0"`
}

type StatsResponse struct {
	TotalResults       int            `json:"total_results" example:"1000"`
	StatusDistribution map[string]int `json:"status_distribution"`
	LatencyStats       LatencyStats   `json:"latency_stats"`
}

type TimeIntervalData struct {
	Timestamp          string         `json:"timestamp" example:"2024-01-01 12:00:00"`
	Count              int            `json:"count" example:"60"`
	SuccessCount       int            `json:"success_count" example:"58"`
	FailureCount       int            `json:"failure_count" example:"2"`
	AvgLatency         float64        `json:"avg_latency" example:"150.5"`
	MinLatency         int            `json:"min_latency" example:"50"`
	MaxLatency         int            `json:"max_latency" example:"500"`
	StatusDistribution map[string]int `json:"status_distribution"`
}

type TimeIntervalResponse struct {
	Interval   string             `json:"interval" example:"1m"`
	Data       []TimeIntervalData `json:"data"`
	Total      int                `json:"total,omitempty"`
	Page       int                `json:"page,omitempty"`
	PageSize   int                `json:"page_size,omitempty"`
	TotalPages int                `json:"total_pages,omitempty"`
}

type NotificationSettings struct {
	ID                    int    `json:"id" example:"1"`
	Type                  string `json:"type" example:"telegram"`
	Enabled               bool   `json:"enabled" example:"true"`
	Token                 string `json:"token,omitempty" example:"123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"`
	ChatID                string `json:"chat_id,omitempty" example:"-1001234567890"`
	WebhookURL            string `json:"webhook_url,omitempty" example:"https://hooks.slack.com/services/..."`
	NotifyOnFailure       bool   `json:"notify_on_failure" example:"true"`
	NotifyOnSuccess       bool   `json:"notify_on_success" example:"false"`
	NotifyOnSlowResponse  bool   `json:"notify_on_slow_response" example:"true"`
	SlowResponseThreshold int    `json:"slow_response_threshold_ms" example:"1000"`
}
