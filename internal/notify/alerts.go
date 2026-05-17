package notify

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"time"
)

const defaultSiteName = "LLM Service Monitor"

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
	if err := modelAlertTemplate.Execute(&html, data); err != nil {
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
	case "missing":
		return modelAlertTone{
			Label:      "Missing",
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

var modelAlertTemplate = template.Must(template.New("model-alert").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Subject}}</title>
  </head>
  <body style="margin:0; padding:0; background:#f7f7f4; color:#202123; font-family:Inter,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Arial,sans-serif;">
    <div style="display:none; max-height:0; overflow:hidden; opacity:0; color:transparent;">{{.Preheader}}</div>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f7f7f4; margin:0; padding:32px 12px 56px;">
      <tr>
        <td align="center">
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="width:100%; max-width:680px; background:#ffffff; border:1px solid #e5e3dc; border-radius:14px; overflow:hidden; box-shadow:0 4px 16px rgba(0,0,0,0.08);">
            <tr>
              <td style="height:4px; line-height:4px; font-size:0; background:#10a37f;">&nbsp;</td>
            </tr>
            <tr>
              <td style="padding:24px 28px 18px; background:#ffffff;">
                <table role="presentation" width="100%" cellspacing="0" cellpadding="0">
                  <tr>
                    <td width="46" valign="top">
                      <table role="presentation" cellspacing="0" cellpadding="0" style="width:38px; height:38px; border-radius:9px; background:#e7f7f1;">
                        <tr>
                          <td align="center" valign="middle" style="height:38px; color:#10a37f; font-size:12px; line-height:12px; font-weight:800;">LLM</td>
                        </tr>
                      </table>
                    </td>
                    <td valign="top">
                      <p style="margin:0; color:#6b7280; font-size:12px; line-height:16px; font-weight:700; letter-spacing:0.04em; text-transform:uppercase;">{{.SiteName}}</p>
                      <p style="margin:3px 0 0; color:#9ca3af; font-size:13px; line-height:18px;">{{.Tone.Eyebrow}}</p>
                    </td>
                  </tr>
                </table>
                <h1 style="margin:22px 0 0; color:#202123; font-size:28px; line-height:34px; font-weight:650; letter-spacing:0;">{{.Subject}}</h1>
                {{if .ModelID}}
                <p style="margin:12px 0 0;">
                  <span style="display:inline-block; padding:7px 10px; border:1px solid #e5e3dc; border-radius:999px; background:#fafaf8; color:#202123; font-size:13px; line-height:16px; font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;">{{.ModelID}}</span>
                </p>
                {{end}}
              </td>
            </tr>
            <tr>
              <td style="padding:0 28px 28px;">
                <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border:1px solid #e5e3dc; border-left:4px solid {{.Tone.Accent}}; border-radius:12px; overflow:hidden; background:#fafaf8;">
                  <tr>
                    <td style="padding:20px 20px 18px;">
                      <p style="margin:0 0 13px;">
                        <span style="display:inline-block; padding:6px 10px; border-radius:999px; background:{{.Tone.AccentSoft}}; color:{{.Tone.Text}}; font-size:12px; line-height:16px; font-weight:700;">{{.Tone.Label}}</span>
                      </p>
                      <p style="margin:0; color:#202123; font-size:17px; line-height:26px;">{{.Summary}}</p>
                    </td>
                    <td width="96" align="right" valign="top" style="padding:20px 20px 18px 8px;">
                      <table role="presentation" cellspacing="0" cellpadding="0" style="width:52px; height:52px; border-radius:13px; background:{{.Tone.AccentSoft}};">
                        <tr>
                          <td align="center" valign="middle" style="height:52px; color:{{.Tone.Text}}; font-size:13px; line-height:13px; font-weight:800;">{{.Tone.Mark}}</td>
                        </tr>
                      </table>
                    </td>
                  </tr>
                </table>
                {{if .Fields}}
                <p style="margin:24px 0 10px; color:#6b7280; font-size:12px; line-height:16px; font-weight:700; letter-spacing:0.04em; text-transform:uppercase;">Details</p>
                <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="border:1px solid #e5e3dc; border-radius:12px; overflow:hidden; background:#ffffff;">
                  {{range .Fields}}
                  <tr>
                    <td style="width:34%; padding:13px 16px; background:#fafaf8; border-bottom:1px solid #e5e3dc; color:#6b7280; font-size:13px; line-height:18px; font-weight:700;">{{.Label}}</td>
                    <td style="padding:13px 16px; border-bottom:1px solid #e5e3dc; color:#202123; font-size:14px; line-height:20px; font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;">{{.Value}}</td>
                  </tr>
                  {{end}}
                </table>
                {{end}}
                {{if .SiteURL}}
                <table role="presentation" cellspacing="0" cellpadding="0" style="margin-top:22px;">
                  <tr>
                    <td style="border-radius:8px; background:#10a37f;">
                      <a href="{{.SiteURL}}" target="_blank" style="display:inline-block; padding:10px 14px; color:#ffffff; font-size:13px; line-height:18px; font-weight:700; text-decoration:none;">Open dashboard</a>
                    </td>
                  </tr>
                </table>
                {{end}}
              </td>
            </tr>
            <tr>
              <td style="background:#fafaf8; border-top:1px solid #e5e3dc; padding:18px 28px 22px;">
                <p style="margin:0; color:#6b7280; font-size:12px; line-height:18px;">Generated automatically by {{.SiteName}}.</p>
                <p style="margin:4px 0 0; color:#9ca3af; font-size:12px; line-height:18px;">Check the dashboard for history, probe details, and recent events.</p>
              </td>
            </tr>
          </table>
          <p style="margin:18px 0 0; color:#9ca3af; font-size:12px; line-height:18px;">&copy; {{.Year}} {{.SiteName}}</p>
          <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="height:32px;">
            <tr>
              <td style="height:32px; line-height:32px; font-size:0;">&nbsp;</td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`))
