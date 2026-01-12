package models

type Domain struct {
	ID   int    `json:"id" example:"1"`
	Name string `json:"name" example:"example.com"`
}

type CheckParams struct {
	Path      string `json:"path,omitempty" example:"/health"`
	Port      int    `json:"port,omitempty" example:"80"`
	Payload   string `json:"payload,omitempty" example:"ping"`
	TimeoutMS int    `json:"timeout_ms,omitempty" example:"5000"`
}

type Check struct {
	ID                 int         `json:"id" example:"1"`
	DomainID           int         `json:"domain_id" example:"1"`
	Type               string      `json:"type" example:"http"`
	IntervalSeconds    int         `json:"interval_seconds" example:"60"`
	Params             CheckParams `json:"params"`
	Enabled            bool        `json:"enabled" example:"true"`
	Frequency          string      `json:"frequency,omitempty" example:"60s"`
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
