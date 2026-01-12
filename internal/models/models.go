package models

type Domain struct {
	ID   int    `json:"id" example:"1"`
	Name string `json:"name" example:"example.com"`
}

type Check struct {
	ID        int    `json:"id" example:"1"`
	DomainID  int    `json:"domain_id" example:"1"`
	Type      string `json:"type" example:"http"`
	Frequency string `json:"frequency" example:"5m"`
	Path      string `json:"path" example:"/"`
}

type Result struct {
	ID         int    `json:"id" example:"1"`
	CheckID    int    `json:"check_id" example:"1"`
	StatusCode int    `json:"status_code" example:"200"`
	DurationMS int    `json:"duration_ms" example:"150"`
	Outcome    string `json:"outcome" example:"success"`
	CreatedAt  string `json:"created_at" example:"2024-01-01T12:00:00Z"`
}

type Check struct {
	ID        int    `json:"id"`
	DomainID  int    `json:"domain_id"`
	Type      string `json:"type"`
	Frequency string `json:"frequency"`
	Path      string `json:"path"`
}

type Result struct {
	ID         int    `json:"id"`
	CheckID    int    `json:"check_id"`
	StatusCode int    `json:"status_code"`
	DurationMS int    `json:"duration_ms"`
	Outcome    string `json:"outcome"`
	CreatedAt  string `json:"created_at"`
}
