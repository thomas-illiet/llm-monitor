package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

// AlertField is one label/value fact displayed in alert email details.
type AlertField struct {
	Label string
	Value string
}

// ModelAlert describes one model lifecycle alert before email rendering.
type ModelAlert struct {
	Type    string
	Subject string
	ModelID string
	Summary string
	Fields  []AlertField
	SentAt  time.Time
}

// modelAlertTone stores visual copy and colors for a model alert type.
type modelAlertTone struct {
	Label      string
	Eyebrow    string
	Accent     string
	AccentSoft string
	Text       string
}

// modelAlertTemplateData is the view model rendered into the HTML template.
type modelAlertTemplateData struct {
	Subject   string
	ModelID   string
	Summary   string
	Preheader string
	Tone      modelAlertTone
	Fields    []AlertField
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

	data := modelAlertTemplateData{
		Subject:   alert.Subject,
		ModelID:   alert.ModelID,
		Summary:   alert.Summary,
		Preheader: alert.Summary,
		Tone:      modelAlertToneFor(alert.Type),
		Fields:    fields,
		Year:      time.Now().UTC().Year(),
	}

	var html bytes.Buffer
	if err := modelAlertTemplate.Execute(&html, data); err != nil {
		return PlainMessage(alert.Subject, modelAlertText(alert.Subject, alert.Summary, fields))
	}

	return Message{
		Subject:  alert.Subject,
		TextBody: modelAlertText(alert.Subject, alert.Summary, fields),
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
func modelAlertText(subject, summary string, fields []AlertField) string {
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
	return strings.TrimSpace(builder.String())
}

// modelAlertToneFor selects the email tone for a lifecycle alert type.
func modelAlertToneFor(alertType string) modelAlertTone {
	switch alertType {
	case "missing":
		return modelAlertTone{
			Label:      "Missing",
			Eyebrow:    "Attention required",
			Accent:     "#b42318",
			AccentSoft: "#fff1f0",
			Text:       "#7a271a",
		}
	case "returned":
		return modelAlertTone{
			Label:      "Recovered",
			Eyebrow:    "Service recovered",
			Accent:     "#0f8f6f",
			AccentSoft: "#e7f7f1",
			Text:       "#075e49",
		}
	case "first_seen":
		return modelAlertTone{
			Label:      "New model",
			Eyebrow:    "Inventory changed",
			Accent:     "#2563eb",
			AccentSoft: "#eff6ff",
			Text:       "#1d4ed8",
		}
	default:
		return modelAlertTone{
			Label:      "Model alert",
			Eyebrow:    "Monitor notification",
			Accent:     "#475467",
			AccentSoft: "#f2f4f7",
			Text:       "#344054",
		}
	}
}

var modelAlertTemplate = template.Must(template.New("model-alert").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Subject}}</title>
  </head>
  <body style="margin:0; padding:0; background:#eef3f1; color:#17211d; font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Arial,sans-serif;">
    <div style="display:none; max-height:0; overflow:hidden; opacity:0; color:transparent;">{{.Preheader}}</div>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#eef3f1; margin:0; padding:32px 12px;">
      <tr>
        <td align="center">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="width:100%; max-width:640px; background:#ffffff; border:1px solid #d8e2dd; border-radius:16px; overflow:hidden;">
            <tr>
              <td style="background:#17211d; padding:28px 30px 24px;">
                <p style="margin:0 0 12px; color:#a8bbb4; font-size:12px; line-height:18px; font-weight:700; letter-spacing:0.08em; text-transform:uppercase;">LLM Service Monitor</p>
                <h1 style="margin:0; color:#ffffff; font-size:28px; line-height:34px; font-weight:700;">{{.Subject}}</h1>
                {{if .ModelID}}<p style="margin:12px 0 0; color:#dce8e3; font-size:15px; line-height:22px;">Model <strong style="color:#ffffff;">{{.ModelID}}</strong></p>{{end}}
              </td>
            </tr>
            <tr>
              <td style="padding:30px;">
                <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                  <tr>
                    <td style="border-left:4px solid {{.Tone.Accent}}; padding-left:16px;">
                      <p style="margin:0 0 12px;">
                        <span style="display:inline-block; padding:6px 10px; border-radius:999px; background:{{.Tone.AccentSoft}}; color:{{.Tone.Text}}; font-size:12px; line-height:16px; font-weight:700;">{{.Tone.Label}}</span>
                      </p>
                      <p style="margin:0; color:#17211d; font-size:17px; line-height:26px;">{{.Summary}}</p>
                    </td>
                  </tr>
                </table>
                {{if .Fields}}
                <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="margin-top:26px; border:1px solid #e5ebe8; border-radius:12px; overflow:hidden;">
                  {{range .Fields}}
                  <tr>
                    <td style="width:34%; padding:13px 16px; background:#f8faf9; border-bottom:1px solid #e5ebe8; color:#66756f; font-size:13px; line-height:18px; font-weight:700;">{{.Label}}</td>
                    <td style="padding:13px 16px; border-bottom:1px solid #e5ebe8; color:#17211d; font-size:14px; line-height:20px; font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;">{{.Value}}</td>
                  </tr>
                  {{end}}
                </table>
                {{end}}
              </td>
            </tr>
            <tr>
              <td style="background:#f8faf9; border-top:1px solid #e5ebe8; padding:18px 30px;">
                <p style="margin:0; color:#7c8b86; font-size:12px; line-height:18px;">{{.Tone.Eyebrow}} from LLM Service Monitor. This message was generated automatically.</p>
              </td>
            </tr>
          </table>
          <p style="margin:18px 0 0; color:#8a9893; font-size:12px; line-height:18px;">&copy; {{.Year}} LLM Service Monitor</p>
        </td>
      </tr>
    </table>
  </body>
</html>`))
