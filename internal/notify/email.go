package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"llmservicemonitor/internal/config"
)

// Notifier sends prepared alert messages.
type Notifier interface {
	Send(message Message) error
}

// Message contains the text and optional HTML bodies for an alert email.
type Message struct {
	Subject  string
	TextBody string
	HTMLBody string
}

// PlainMessage wraps a text-only email in the richer message shape.
func PlainMessage(subject, body string) Message {
	return Message{Subject: subject, TextBody: body}
}

// SMTPNotifier delivers alert messages through SMTP.
type SMTPNotifier struct {
	cfg      config.SMTPConfig
	password string
	logger   *slog.Logger
}

// NewSMTPNotifier prepares SMTP settings and reads any mounted password file.
func NewSMTPNotifier(cfg config.SMTPConfig, logger *slog.Logger) (*SMTPNotifier, error) {
	password, err := config.ReadSecret(cfg.Password, cfg.PasswordFile)
	if err != nil {
		return nil, err
	}
	return &SMTPNotifier{cfg: cfg, password: password, logger: logger}, nil
}

// Send delivers an alert email, or logs a skip when SMTP is disabled.
func (n *SMTPNotifier) Send(message Message) error {
	if !n.cfg.Enabled {
		n.logger.Info("smtp disabled, alert skipped", "subject", message.Subject)
		return nil
	}
	addr := net.JoinHostPort(n.cfg.Host, fmt.Sprint(n.cfg.Port))
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	if n.cfg.StartTLS {
		tlsConfig := &tls.Config{
			ServerName:         n.cfg.Host,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: n.cfg.InsecureSkipVerify,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return err
		}
	}
	if n.cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", n.cfg.Username, n.password, n.cfg.Host)); err != nil {
			return err
		}
	}
	if err := client.Mail(n.cfg.From); err != nil {
		return err
	}
	for _, recipient := range n.cfg.To {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	rawMessage, err := buildEmailMessage(n.cfg.From, n.cfg.To, message)
	if err != nil {
		return err
	}
	if _, err := writer.Write(rawMessage); err != nil {
		return err
	}
	return writer.Close()
}

// buildEmailMessage renders a MIME email with text-only or multipart content.
func buildEmailMessage(from string, to []string, message Message) ([]byte, error) {
	var buf bytes.Buffer
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(to, ", "),
		"Subject: " + mime.QEncoding.Encode("utf-8", message.Subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
	}

	textBody := message.TextBody
	if strings.TrimSpace(textBody) == "" {
		textBody = "This alert does not include a text body."
	}

	if strings.TrimSpace(message.HTMLBody) == "" {
		headers = append(headers,
			"Content-Type: text/plain; charset=utf-8",
			"Content-Transfer-Encoding: quoted-printable",
		)
		writeHeaders(&buf, headers)
		if err := writeQuotedPrintable(&buf, textBody); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	writer := multipart.NewWriter(&buf)
	headers = append(headers, `Content-Type: multipart/alternative; boundary="`+writer.Boundary()+`"`)
	writeHeaders(&buf, headers)

	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=utf-8")
	textHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	textPart, err := writer.CreatePart(textHeader)
	if err != nil {
		return nil, err
	}
	if err := writeQuotedPrintable(textPart, textBody); err != nil {
		return nil, err
	}

	htmlHeader := textproto.MIMEHeader{}
	htmlHeader.Set("Content-Type", "text/html; charset=utf-8")
	htmlHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	htmlPart, err := writer.CreatePart(htmlHeader)
	if err != nil {
		return nil, err
	}
	if err := writeQuotedPrintable(htmlPart, message.HTMLBody); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeHeaders writes CRLF-delimited email headers.
func writeHeaders(buf *bytes.Buffer, headers []string) {
	for _, header := range headers {
		buf.WriteString(header)
		buf.WriteString("\r\n")
	}
	buf.WriteString("\r\n")
}

// writeQuotedPrintable encodes one email body part.
func writeQuotedPrintable(writer io.Writer, body string) error {
	quotedWriter := quotedprintable.NewWriter(writer)
	if _, err := io.WriteString(quotedWriter, body); err != nil {
		_ = quotedWriter.Close()
		return err
	}
	return quotedWriter.Close()
}
