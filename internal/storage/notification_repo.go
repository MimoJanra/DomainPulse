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

func (r *NotificationRepo) GetAll() ([]models.NotificationSettings, error) {
	rows, err := r.db.Query(`
		SELECT id, type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success
		FROM notification_settings
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []models.NotificationSettings
	for rows.Next() {
		var s models.NotificationSettings
		var token, chatID, webhookURL sql.NullString
		if err := rows.Scan(&s.ID, &s.Type, &s.Enabled, &token, &chatID, &webhookURL, &s.NotifyOnFailure, &s.NotifyOnSuccess); err != nil {
			return nil, err
		}
		if token.Valid {
			s.Token = token.String
		}
		if chatID.Valid {
			s.ChatID = chatID.String
		}
		if webhookURL.Valid {
			s.WebhookURL = webhookURL.String
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (r *NotificationRepo) GetByID(id int) (models.NotificationSettings, error) {
	row := r.db.QueryRow(`
		SELECT id, type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success
		FROM notification_settings
		WHERE id = ?
	`, id)
	var s models.NotificationSettings
	var token, chatID, webhookURL sql.NullString
	err := row.Scan(&s.ID, &s.Type, &s.Enabled, &token, &chatID, &webhookURL, &s.NotifyOnFailure, &s.NotifyOnSuccess)
	if err != nil {
		return s, err
	}
	if token.Valid {
		s.Token = token.String
	}
	if chatID.Valid {
		s.ChatID = chatID.String
	}
	if webhookURL.Valid {
		s.WebhookURL = webhookURL.String
	}
	return s, nil
}

func (r *NotificationRepo) GetEnabled() ([]models.NotificationSettings, error) {
	rows, err := r.db.Query(`
		SELECT id, type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success
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
		var s models.NotificationSettings
		var token, chatID, webhookURL sql.NullString
		if err := rows.Scan(&s.ID, &s.Type, &s.Enabled, &token, &chatID, &webhookURL, &s.NotifyOnFailure, &s.NotifyOnSuccess); err != nil {
			return nil, err
		}
		if token.Valid {
			s.Token = token.String
		}
		if chatID.Valid {
			s.ChatID = chatID.String
		}
		if webhookURL.Valid {
			s.WebhookURL = webhookURL.String
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

func (r *NotificationRepo) Add(settings models.NotificationSettings) (models.NotificationSettings, error) {
	if settings.Type != "telegram" && settings.Type != "slack" {
		return models.NotificationSettings{}, fmt.Errorf("unsupported notification type: %s", settings.Type)
	}

	enabledInt := 0
	if settings.Enabled {
		enabledInt = 1
	}
	notifyOnFailureInt := 0
	if settings.NotifyOnFailure {
		notifyOnFailureInt = 1
	}
	notifyOnSuccessInt := 0
	if settings.NotifyOnSuccess {
		notifyOnSuccessInt = 1
	}

	res, err := r.db.Exec(`
		INSERT INTO notification_settings(type, enabled, token, chat_id, webhook_url, notify_on_failure, notify_on_success)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, settings.Type, enabledInt, settings.Token, settings.ChatID, settings.WebhookURL, notifyOnFailureInt, notifyOnSuccessInt)
	if err != nil {
		return models.NotificationSettings{}, err
	}
	id, _ := res.LastInsertId()
	settings.ID = int(id)
	return settings, nil
}

func (r *NotificationRepo) Update(id int, settings models.NotificationSettings) error {
	if settings.Type != "telegram" && settings.Type != "slack" {
		return fmt.Errorf("unsupported notification type: %s", settings.Type)
	}

	enabledInt := 0
	if settings.Enabled {
		enabledInt = 1
	}
	notifyOnFailureInt := 0
	if settings.NotifyOnFailure {
		notifyOnFailureInt = 1
	}
	notifyOnSuccessInt := 0
	if settings.NotifyOnSuccess {
		notifyOnSuccessInt = 1
	}

	_, err := r.db.Exec(`
		UPDATE notification_settings
		SET type = ?, enabled = ?, token = ?, chat_id = ?, webhook_url = ?, notify_on_failure = ?, notify_on_success = ?
		WHERE id = ?
	`, settings.Type, enabledInt, settings.Token, settings.ChatID, settings.WebhookURL, notifyOnFailureInt, notifyOnSuccessInt, id)
	return err
}

func (r *NotificationRepo) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM notification_settings WHERE id = ?`, id)
	return err
}

func (r *NotificationRepo) SetEnabled(id int, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := r.db.Exec(`UPDATE notification_settings SET enabled = ? WHERE id = ?`, enabledInt, id)
	return err
}
