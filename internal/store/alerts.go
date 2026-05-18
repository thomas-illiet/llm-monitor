package store

import "context"

const emailAlertExistsSQL = `SELECT EXISTS(SELECT 1 FROM email_alerts WHERE alert_key=$1 AND error='')`

const recordEmailAlertSQL = `
	INSERT INTO email_alerts(alert_key, model_id, alert_type, sent_at, subject, recipients, error)
	VALUES($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT(alert_key) DO UPDATE SET
		model_id=EXCLUDED.model_id,
		alert_type=EXCLUDED.alert_type,
		sent_at=EXCLUDED.sent_at,
		subject=EXCLUDED.subject,
		recipients=EXCLUDED.recipients,
		error=EXCLUDED.error
	WHERE email_alerts.error <> ''
`

// EmailAlertExists checks whether an alert key already has a successful delivery.
func (s *Store) EmailAlertExists(ctx context.Context, key string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, emailAlertExistsSQL, key).Scan(&exists)
	return exists, err
}

// RecordEmailAlert records that a model alert email was sent or attempted.
func (s *Store) RecordEmailAlert(ctx context.Context, record EmailAlertRecord) error {
	recipients := record.To
	if recipients == nil {
		recipients = []string{}
	}
	_, err := s.pool.Exec(ctx, recordEmailAlertSQL, record.AlertKey, record.ModelID, record.Type, record.SentAt, record.Subject, recipients, record.Error)
	return err
}

// RecentAlerts returns the latest alert emails for model lifecycle changes.
func (s *Store) RecentAlerts(ctx context.Context, limit int) ([]RecentAlert, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT model_id, alert_type, sent_at, subject, recipients, error
		FROM email_alerts
		ORDER BY sent_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []RecentAlert
	for rows.Next() {
		var alert RecentAlert
		if err := rows.Scan(&alert.ModelID, &alert.Type, &alert.SentAt, &alert.Subject, &alert.Recipients, &alert.Error); err != nil {
			return nil, err
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}
