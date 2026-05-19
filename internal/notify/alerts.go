package notify

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

const (
	defaultSiteName        = "LLM Service Monitor"
	modelAlertTemplateName = "model_alert.html"
	modelAlertTemplatePath = "templates/" + modelAlertTemplateName
)

//go:embed templates/model_alert.html
var modelAlertTemplates embed.FS

// AlertField is one label/value fact displayed in alert email details.
type AlertField struct {
	Label string
	Value string
}

// ModelAlert describes one model lifecycle alert before email rendering.
type ModelAlert struct {
	Type     string
	Subject  string
	ModelID  string
	Summary  string
	Fields   []AlertField
	SentAt   time.Time
	SiteName string
	SiteURL  string
}

// modelAlertTone stores visual copy and colors for a model alert type.
type modelAlertTone struct {
	Label      string
	Eyebrow    string
	Accent     string
	AccentSoft string
	Text       string
	Mark       string
}

// modelAlertTemplateData is the view model rendered into the HTML template.
type modelAlertTemplateData struct {
	Subject   string
	ModelID   string
	Summary   string
	Preheader string
	Tone      modelAlertTone
	Fields    []AlertField
	SiteName  string
	SiteURL   string
	Year      int
}

// NewModelAlertMessage renders model lifecycle alerts as HTML with a text fallback.
func NewModelAlertMessage(alert ModelAlert) Message {
	fields := compactFields(alert.Fields)
	if !alert.SentAt.IsZero() {
		fields = append(fields, AlertField{
			Label: "Generated at",
			Value: alert.SentAt.Format(time.RFC3339),
		})
	}
	siteName := strings.TrimSpace(alert.SiteName)
	if siteName == "" {
		siteName = defaultSiteName
	}
	siteURL := strings.TrimSpace(alert.SiteURL)

	data := modelAlertTemplateData{
		Subject:   alert.Subject,
		ModelID:   alert.ModelID,
		Summary:   alert.Summary,
		Preheader: alert.Summary,
		Tone:      modelAlertToneFor(alert.Type),
		Fields:    fields,
		SiteName:  siteName,
		SiteURL:   siteURL,
		Year:      time.Now().UTC().Year(),
	}

	var html bytes.Buffer
	if err := modelAlertTemplate.ExecuteTemplate(&html, modelAlertTemplateName, data); err != nil {
		return PlainMessage(alert.Subject, modelAlertText(alert.Subject, alert.Summary, fields, siteURL))
	}

	return Message{
		Subject:  alert.Subject,
		TextBody: modelAlertText(alert.Subject, alert.Summary, fields, siteURL),
		HTMLBody: html.String(),
	}
}

// compactFields drops empty alert fields before rendering.
func compactFields(fields []AlertField) []AlertField {
	cleaned := make([]AlertField, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.Label) == "" || strings.TrimSpace(field.Value) == "" {
			continue
		}
		cleaned = append(cleaned, field)
	}
	return cleaned
}

// modelAlertText renders the plain-text fallback for an alert email.
func modelAlertText(subject, summary string, fields []AlertField, siteURL string) string {
	var builder strings.Builder
	builder.WriteString(subject)
	builder.WriteString("\n\n")
	builder.WriteString(summary)
	if len(fields) > 0 {
		builder.WriteString("\n\nDetails\n")
	}
	for _, field := range fields {
		fmt.Fprintf(&builder, "%s: %s\n", field.Label, field.Value)
	}
	if strings.TrimSpace(siteURL) != "" {
		fmt.Fprintf(&builder, "\nDashboard: %s", strings.TrimSpace(siteURL))
	}
	return strings.TrimSpace(builder.String())
}

// modelAlertToneFor selects the email tone for a lifecycle alert type.
func modelAlertToneFor(alertType string) modelAlertTone {
	switch alertType {
	case "inactive", "missing":
		return modelAlertTone{
			Label:      "Inactive",
			Eyebrow:    "Attention required",
			Accent:     "#b42318",
			AccentSoft: "#fff7ed",
			Text:       "#9a3412",
			Mark:       "!",
		}
	case "returned":
		return modelAlertTone{
			Label:      "Recovered",
			Eyebrow:    "Service recovered",
			Accent:     "#10a37f",
			AccentSoft: "#e7f7f1",
			Text:       "#0f8f6f",
			Mark:       "OK",
		}
	case "first_seen":
		return modelAlertTone{
			Label:      "New model",
			Eyebrow:    "Inventory changed",
			Accent:     "#2563eb",
			AccentSoft: "#eff6ff",
			Text:       "#1d4ed8",
			Mark:       "NEW",
		}
	default:
		return modelAlertTone{
			Label:      "Model alert",
			Eyebrow:    "Monitor notification",
			Accent:     "#475467",
			AccentSoft: "#f2f4f7",
			Text:       "#344054",
			Mark:       "LLM",
		}
	}
}

var modelAlertTemplate = template.Must(template.ParseFS(modelAlertTemplates, modelAlertTemplatePath))
