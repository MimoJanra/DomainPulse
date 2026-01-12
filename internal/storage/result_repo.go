package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type ResultRepo struct {
	db *sql.DB
}

func NewResultRepo(db *sql.DB) *ResultRepo { return &ResultRepo{db: db} }

func (r *ResultRepo) Add(res models.Result) error {
	timestamp := res.CreatedAt
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	_, err := r.db.Exec(`
		INSERT INTO results(check_id, status, status_code, duration_ms, outcome, error_message, created_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, res.CheckID, res.Status, res.StatusCode, res.DurationMS, res.Outcome, res.ErrorMessage, timestamp)
	return err
}

func (r *ResultRepo) GetByCheckID(checkID int) ([]models.Result, error) {
	rows, err := r.db.Query(`
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		WHERE check_id = ?
		ORDER BY created_at DESC
	`, checkID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Result
	for rows.Next() {
		var res models.Result
		if err := rows.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *ResultRepo) GetAll() ([]models.Result, error) {
	rows, err := r.db.Query(`
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.Result
	for rows.Next() {
		var res models.Result
		if err := rows.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

func (r *ResultRepo) GetByID(id int) (models.Result, error) {
	row := r.db.QueryRow(`
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		WHERE id = ?
	`, id)
	var res models.Result
	err := row.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt)
	return res, err
}

func (r *ResultRepo) GetByCheckIDWithPagination(checkID int, from, to *time.Time, page, pageSize int) ([]models.Result, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	offset := (page - 1) * pageSize

	query := `
		SELECT id, check_id, status, status_code, duration_ms, outcome, error_message, created_at
		FROM results
		WHERE check_id = ?
	`
	args := []any{checkID}

	if from != nil {
		query += " AND datetime(created_at) >= datetime(?)"
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		query += " AND datetime(created_at) <= datetime(?)"
		args = append(args, to.Format(time.RFC3339))
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []models.Result
	for rows.Next() {
		var res models.Result
		if err := rows.Scan(&res.ID, &res.CheckID, &res.Status, &res.StatusCode, &res.DurationMS, &res.Outcome, &res.ErrorMessage, &res.CreatedAt); err != nil {
			return nil, 0, err
		}
		results = append(results, res)
	}

	countQuery := "SELECT COUNT(*) FROM results WHERE check_id = ?"
	countArgs := []any{checkID}
	if from != nil {
		countQuery += " AND datetime(created_at) >= datetime(?)"
		countArgs = append(countArgs, from.Format(time.RFC3339))
	}
	if to != nil {
		countQuery += " AND datetime(created_at) <= datetime(?)"
		countArgs = append(countArgs, to.Format(time.RFC3339))
	}

	var total int
	err = r.db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if results == nil {
		results = []models.Result{}
	}

	return results, total, rows.Err()
}

type Stats struct {
	TotalResults       int                 `json:"total_results"`
	StatusDistribution map[string]int      `json:"status_distribution"`
	LatencyStats       models.LatencyStats `json:"latency_stats"`
}

func (r *ResultRepo) GetStats(checkID int, from, to *time.Time) (Stats, error) {
	var stats Stats
	stats.StatusDistribution = make(map[string]int)

	query := "SELECT status, duration_ms FROM results WHERE check_id = ?"
	args := []any{checkID}

	if from != nil {
		query += " AND datetime(created_at) >= datetime(?)"
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		query += " AND datetime(created_at) <= datetime(?)"
		args = append(args, to.Format(time.RFC3339))
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	var durations []int
	for rows.Next() {
		var status string
		var duration int
		if err := rows.Scan(&status, &duration); err != nil {
			return stats, err
		}
		stats.StatusDistribution[status]++
		stats.TotalResults++
		durations = append(durations, duration)
	}

	if err := rows.Err(); err != nil {
		return stats, err
	}

	if len(durations) == 0 {
		return stats, nil
	}

	stats.LatencyStats = calculateLatencyStats(durations)

	return stats, nil
}

func calculateLatencyStats(durations []int) models.LatencyStats {
	if len(durations) == 0 {
		return models.LatencyStats{}
	}

	sorted := make([]int, len(durations))
	copy(sorted, durations)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var sum int
	min, max := sorted[0], sorted[0]
	for _, d := range sorted {
		sum += d
		if d < min {
			min = d
		}
		if d > max {
			max = d
		}
	}

	avg := float64(sum) / float64(len(sorted))
	median := float64(sorted[len(sorted)/2])
	if len(sorted)%2 == 0 {
		median = float64(sorted[len(sorted)/2-1]+sorted[len(sorted)/2]) / 2.0
	}

	p95Idx := int(float64(len(sorted)) * 0.95)
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}
	p95 := float64(sorted[p95Idx])

	p99Idx := int(float64(len(sorted)) * 0.99)
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}
	p99 := float64(sorted[p99Idx])

	return models.LatencyStats{
		Min:    min,
		Max:    max,
		Avg:    avg,
		Median: median,
		P95:    p95,
		P99:    p99,
	}
}

func (r *ResultRepo) GetByTimeInterval(checkID int, interval string, from, to *time.Time, page, pageSize int) ([]models.TimeIntervalData, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	offset := (page - 1) * pageSize
	var timeTruncate string

	switch interval {
	case "1m":
		timeTruncate = "strftime('%Y-%m-%d %H:%M:00', created_at)"
	case "5m":
		timeTruncate = "strftime('%Y-%m-%d %H:', created_at) || printf('%02d', (CAST(strftime('%M', created_at) AS INTEGER) / 5) * 5) || ':00'"
	case "1h":
		timeTruncate = "strftime('%Y-%m-%d %H:00:00', created_at)"
	default:
		return nil, 0, fmt.Errorf("unsupported interval: %s. Supported: 1m, 5m, 1h", interval)
	}

	query := fmt.Sprintf(`
		SELECT 
			%s as time_bucket,
			COUNT(*) as count,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as failure_count,
			AVG(duration_ms) as avg_latency,
			MIN(duration_ms) as min_latency,
			MAX(duration_ms) as max_latency
		FROM results
		WHERE check_id = ?
	`, timeTruncate)

	args := []any{checkID}

	if from != nil {
		query += " AND datetime(created_at) >= datetime(?)"
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		query += " AND datetime(created_at) <= datetime(?)"
		args = append(args, to.Format(time.RFC3339))
	}

	query += " GROUP BY time_bucket ORDER BY time_bucket ASC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []models.TimeIntervalData
	for rows.Next() {
		var data models.TimeIntervalData
		var timestamp string
		var successCount, failureCount int
		var avgLatency sql.NullFloat64

		if err := rows.Scan(&timestamp, &data.Count, &successCount, &failureCount, &avgLatency, &data.MinLatency, &data.MaxLatency); err != nil {
			return nil, 0, err
		}

		data.Timestamp = timestamp
		data.SuccessCount = successCount
		data.FailureCount = failureCount
		if avgLatency.Valid {
			data.AvgLatency = avgLatency.Float64
		}
		data.StatusDistribution = make(map[string]int)

		statusQuery := fmt.Sprintf(`
			SELECT status, COUNT(*) 
			FROM results 
			WHERE check_id = ? AND %s = ?
		`, timeTruncate)
		statusArgs := []any{checkID, timestamp}
		if from != nil {
			statusQuery += " AND datetime(created_at) >= datetime(?)"
			statusArgs = append(statusArgs, from.Format(time.RFC3339))
		}
		if to != nil {
			statusQuery += " AND datetime(created_at) <= datetime(?)"
			statusArgs = append(statusArgs, to.Format(time.RFC3339))
		}
		statusQuery += " GROUP BY status"

		statusRows, err := r.db.Query(statusQuery, statusArgs...)
		if err == nil {
			for statusRows.Next() {
				var status string
				var count int
				if err := statusRows.Scan(&status, &count); err == nil {
					data.StatusDistribution[status] = count
				}
			}
			statusRows.Close()
		}

		results = append(results, data)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT %s) FROM results WHERE check_id = ?", timeTruncate)
	countArgs := []any{checkID}
	if from != nil {
		countQuery += " AND datetime(created_at) >= datetime(?)"
		countArgs = append(countArgs, from.Format(time.RFC3339))
	}
	if to != nil {
		countQuery += " AND datetime(created_at) <= datetime(?)"
		countArgs = append(countArgs, to.Format(time.RFC3339))
	}

	var total int
	err = r.db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if results == nil {
		results = []models.TimeIntervalData{}
	}

	return results, total, rows.Err()
}

func (r *ResultRepo) GetRecentDataForAllChecks(from, to *time.Time, page, pageSize int) ([]models.TimeIntervalData, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	offset := (page - 1) * pageSize
	timeTruncate := "strftime('%Y-%m-%d %H:%M:00', created_at)"

	query := fmt.Sprintf(`
		SELECT 
			%s as time_bucket,
			COUNT(*) as count,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status != 'success' THEN 1 ELSE 0 END) as failure_count,
			AVG(duration_ms) as avg_latency,
			MIN(duration_ms) as min_latency,
			MAX(duration_ms) as max_latency
		FROM results
		WHERE 1=1
	`, timeTruncate)

	args := []any{}
	if from != nil {
		query += " AND datetime(created_at) >= datetime(?)"
		args = append(args, from.Format(time.RFC3339))
	}
	if to != nil {
		query += " AND datetime(created_at) <= datetime(?)"
		args = append(args, to.Format(time.RFC3339))
	}

	query += " GROUP BY time_bucket ORDER BY time_bucket ASC LIMIT ? OFFSET ?"
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []models.TimeIntervalData
	for rows.Next() {
		var data models.TimeIntervalData
		var timestamp string
		var successCount, failureCount int
		var avgLatency sql.NullFloat64

		if err := rows.Scan(&timestamp, &data.Count, &successCount, &failureCount, &avgLatency, &data.MinLatency, &data.MaxLatency); err != nil {
			return nil, 0, err
		}

		data.Timestamp = timestamp
		data.SuccessCount = successCount
		data.FailureCount = failureCount
		if avgLatency.Valid {
			data.AvgLatency = avgLatency.Float64
		}
		data.StatusDistribution = make(map[string]int)

		statusQuery := fmt.Sprintf(`
			SELECT status, COUNT(*) 
			FROM results 
			WHERE %s = ?
		`, timeTruncate)
		statusArgs := []any{timestamp}
		if from != nil {
			statusQuery += " AND datetime(created_at) >= datetime(?)"
			statusArgs = append(statusArgs, from.Format(time.RFC3339))
		}
		if to != nil {
			statusQuery += " AND datetime(created_at) <= datetime(?)"
			statusArgs = append(statusArgs, to.Format(time.RFC3339))
		}
		statusQuery += " GROUP BY status"

		statusRows, err := r.db.Query(statusQuery, statusArgs...)
		if err == nil {
			for statusRows.Next() {
				var status string
				var count int
				if err := statusRows.Scan(&status, &count); err == nil {
					data.StatusDistribution[status] = count
				}
			}
			statusRows.Close()
		}

		results = append(results, data)
	}

	countQuery := "SELECT COUNT(DISTINCT " + timeTruncate + ") FROM results WHERE 1=1"
	countArgs := []any{}
	if from != nil {
		countQuery += " AND datetime(created_at) >= datetime(?)"
		countArgs = append(countArgs, from.Format(time.RFC3339))
	}
	if to != nil {
		countQuery += " AND datetime(created_at) <= datetime(?)"
		countArgs = append(countArgs, to.Format(time.RFC3339))
	}

	var total int
	err = r.db.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	if results == nil {
		results = []models.TimeIntervalData{}
	}

	return results, total, rows.Err()
}
