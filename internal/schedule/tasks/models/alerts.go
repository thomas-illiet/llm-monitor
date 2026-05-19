package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"llmservicemonitor/internal/notify"
	"llmservicemonitor/internal/store"
)

func (s *service) sendInactiveModelAlerts(ctx context.Context, now time.Time) {
	inactive, err := s.store.InactiveModelsForAlert(ctx, s.cfg.Models.AbsenceAlertAfter.Duration, now)
	if err != nil {
		s.logger.Error("load inactive models for alert", "error", err)
		return
	}
	for _, model := range inactive {
		if model.MissingSince == nil {
			continue
		}
		threshold := formatAlertDuration(s.cfg.Models.AbsenceAlertAfter.Duration)
		body := fmt.Sprintf("Model %s has been inactive since %s, which is longer than %s.", model.ModelID, model.MissingSince.Format(time.RFC3339), threshold)
		s.sendModelAlert(ctx, modelAlertKey("inactive", model.ModelID, *model.MissingSince), model.ModelID, "inactive", "LLM model inactive for more than 24h", body, s.modelAlertFields(model.ModelID,
			notify.AlertField{Label: "Inactive since", Value: model.MissingSince.Format(time.RFC3339)},
			notify.AlertField{Label: "Alert threshold", Value: threshold},
		))
	}
}

func (s *service) modelAlertFields(modelID string, fields ...notify.AlertField) []notify.AlertField {
	base := []notify.AlertField{{Label: "Model", Value: modelID}}
	if s.cfg.Target.Name != "" {
		base = append(base, notify.AlertField{Label: "Target", Value: s.cfg.Target.Name})
	}
	return append(base, fields...)
}

func formatAlertDuration(duration time.Duration) string {
	if duration < 0 {
		duration = -duration
	}
	duration = duration.Round(time.Second)
	if duration == 0 {
		return "0s"
	}

	var parts []string
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	duration -= minutes * time.Minute
	seconds := duration / time.Second

	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

func (s *service) sendModelAlert(ctx context.Context, key, modelID, alertType, subject, body string, fields []notify.AlertField) {
	exists, err := s.store.EmailAlertExists(ctx, key)
	if err != nil {
		s.logger.Error("check email dedupe", "error", err, "key", key)
		return
	}
	if exists {
		return
	}
	sentAt := time.Now().UTC()
	err = s.notifier.Send(notify.NewModelAlertMessage(notify.ModelAlert{
		Type:     alertType,
		Subject:  subject,
		ModelID:  modelID,
		Summary:  body,
		Fields:   fields,
		SentAt:   sentAt,
		SiteName: s.cfg.Dashboard.SiteName,
		SiteURL:  s.cfg.Dashboard.SiteURL,
	}))
	record := store.EmailAlertRecord{
		AlertKey: key,
		ModelID:  modelID,
		Type:     alertType,
		SentAt:   sentAt,
		Subject:  subject,
		To:       s.cfg.SMTP.To,
	}
	if err != nil {
		record.Error = err.Error()
		s.logger.Error("send model alert", "error", err, "model", modelID, "type", alertType)
		if recordErr := s.store.RecordEmailAlert(ctx, record); recordErr != nil {
			s.logger.Error("record email alert", "error", recordErr)
		}
		s.recordModelEvent(ctx, store.ModelEventRecord{
			ModelID:    modelID,
			EventType:  "alert_failed",
			Source:     "email_alert",
			Severity:   "error",
			Status:     "error",
			Capability: "unknown",
			Title:      "Email alert failed",
			Message:    fmt.Sprintf("Alert %s could not be sent for model %s.", alertType, modelID),
			Details: map[string]any{
				"alert_key":  key,
				"alert_type": alertType,
				"subject":    subject,
				"error":      err.Error(),
			},
		})
		return
	}
	if err := s.store.RecordEmailAlert(ctx, record); err != nil {
		s.logger.Error("record email alert", "error", err)
	}
	s.recordModelEvent(ctx, store.ModelEventRecord{
		ModelID:    modelID,
		EventType:  "alert_sent",
		Source:     "email_alert",
		Severity:   "info",
		Status:     "ok",
		Capability: "unknown",
		Title:      "Email alert sent",
		Message:    fmt.Sprintf("Alert %s was sent for model %s.", alertType, modelID),
		Details: map[string]any{
			"alert_key":  key,
			"alert_type": alertType,
			"subject":    subject,
			"recipients": s.cfg.SMTP.To,
		},
	})
}
