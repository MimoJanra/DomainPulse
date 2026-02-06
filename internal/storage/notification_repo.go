package storage

import (
	"database/sql"
	"fmt"

	"github.com/MimoJanra/DomainPulse/internal/models"
)

type NotificationRepo struct {
	db *sql.DB
}

func NewNotificationRepo(db *sql.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

type notifScanner interface {
	Scan(dest ...any) error
}

func scanNotificationSettings(s notifScanner) (models.NotificationSettings, error) {
	var ns models.NotificationSettings
	var token, chatID, webhookURL sql.NullString
	var slowThreshold sql.NullInt64
	if err := s.Scan(&ns.ID, &ns.Type, &ns.Enabled, &token, &chatID, &webhookURL, &ns.NotifyOnFailure, &ns.NotifyOnSuccess, &ns.NotifyOnSlowResponse, &slowThreshold); err != nil {
		return models.NotificationSettings{}, err
	}
	if token.Valid {
		ns.Token = token.String
	}
	if chatID.Valid {
		ns.ChatID = chatID.String
	}
	if webhookURL.Valid {
		ns.WebhookURL = webhookURL.String
	}
	if slowThreshold.Valid {
		ns.SlowResponseThreshold = int(slowThreshold.Int64)
	}
	return ns, nil
}

func (r *NotificationRepo) GetAll() ([]models.NotificationSettings, error) {
	rows, err := r.db.Query(`
		SELECT id, type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success, notify_on_slow_response, slow_response_threshold_ms
		FROM notification_settings
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.NotificationSettings
	for rows.Next() {
		s, err := scanNotificationSettings(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (r *NotificationRepo) GetByID(id int) (models.NotificationSettings, error) {
	row := r.db.QueryRow(`
		SELECT id, type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success, notify_on_slow_response, slow_response_threshold_ms
		FROM notification_settings
		WHERE id = ?
	`, id)
	return scanNotificationSettings(row)
}

func (r *NotificationRepo) GetEnabled() ([]models.NotificationSettings, error) {
	rows, err := r.db.Query(`
		SELECT id, type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success, notify_on_slow_response, slow_response_threshold_ms
		FROM notification_settings
		WHERE enabled = 1
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.NotificationSettings
	for rows.Next() {
		s, err := scanNotificationSettings(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (r *NotificationRepo) Add(settings models.NotificationSettings) (models.NotificationSettings, error) {
	if settings.Type != "telegram" && settings.Type != "slack" {
		return models.NotificationSettings{}, fmt.Errorf("unsupported notification type: %s", settings.Type)
	}

	res, err := r.db.Exec(`
		INSERT INTO notification_settings(type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success, notify_on_slow_response, slow_response_threshold_ms)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, settings.Type, boolToInt(settings.Enabled), settings.Token, settings.ChatID, settings.WebhookURL,
		boolToInt(settings.NotifyOnFailure), boolToInt(settings.NotifyOnSuccess), boolToInt(settings.NotifyOnSlowResponse), settings.SlowResponseThreshold)
	if err != nil {
		return models.NotificationSettings{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.NotificationSettings{}, fmt.Errorf("last insert id: %w", err)
	}
	settings.ID = int(id)
	return settings, nil
}

func (r *NotificationRepo) Update(id int, settings models.NotificationSettings) error {
	if settings.Type != "telegram" && settings.Type != "slack" {
		return fmt.Errorf("unsupported notification type: %s", settings.Type)
	}

	_, err := r.db.Exec(`
		UPDATE notification_settings
		SET type = ?, enabled = ?, token = ?, chat_id = ?, webhook_url = ?, notify_on_failure = ?, notify_on_success = ?, notify_on_slow_response = ?, slow_response_threshold_ms = ?
		WHERE id = ?
	`, settings.Type, boolToInt(settings.Enabled), settings.Token, settings.ChatID, settings.WebhookURL,
		boolToInt(settings.NotifyOnFailure), boolToInt(settings.NotifyOnSuccess), boolToInt(settings.NotifyOnSlowResponse), settings.SlowResponseThreshold, id)
	return err
}

func (r *NotificationRepo) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM notification_settings WHERE id = ?`, id)
	return err
}

func (r *NotificationRepo) SetEnabled(id int, enabled bool) error {
	_, err := r.db.Exec(`UPDATE notification_settings SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	return err
}
