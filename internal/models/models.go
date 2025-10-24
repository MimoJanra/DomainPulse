package models

type Domain struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
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
