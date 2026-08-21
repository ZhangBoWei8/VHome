package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"vhome/internal/model"
)

type smtpConfig struct {
	Host      string
	Port      uint16
	Security  model.SMTPSecurity
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

func sendSMTPMessage(ctx context.Context, config smtpConfig, recipient, subject, plainBody, htmlBody string) error {
	address := net.JoinHostPort(config.Host, strconv.Itoa(int(config.Port)))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	tlsConfig := &tls.Config{ServerName: config.Host, MinVersion: tls.VersionTLS12}

	var client *smtp.Client
	var rawConnection net.Conn
	if config.Security == model.SMTPSecurityTLS {
		connection, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
		if err != nil {
			return fmt.Errorf("dial SMTP TLS: %w", err)
		}
		client, err = smtp.NewClient(connection, config.Host)
		if err != nil {
			_ = connection.Close()
			return fmt.Errorf("create SMTP TLS client: %w", err)
		}
		rawConnection = connection
	} else {
		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return fmt.Errorf("dial SMTP STARTTLS: %w", err)
		}
		client, err = smtp.NewClient(connection, config.Host)
		if err != nil {
			_ = connection.Close()
			return fmt.Errorf("create SMTP client: %w", err)
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
		rawConnection = connection
	}
	defer client.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = rawConnection.SetDeadline(deadline)
	}
	authenticator := smtp.PlainAuth("", config.Username, config.Password, config.Host)
	if err := client.Auth(authenticator); err != nil {
		return fmt.Errorf("authenticate SMTP: %w", err)
	}
	if err := client.Mail(config.FromEmail); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(recipient); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	message := buildSMTPMessage(config, recipient, subject, plainBody, htmlBody)
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP session: %w", err)
	}
	return nil
}

func buildSMTPMessage(config smtpConfig, recipient, subject, plainBody, htmlBody string) []byte {
	boundary := "vhome-notification-boundary"
	fromName := mime.QEncoding.Encode("UTF-8", config.FromName)
	encodedSubject := mime.QEncoding.Encode("UTF-8", subject)
	var message bytes.Buffer
	fmt.Fprintf(&message, "From: %s <%s>\r\n", fromName, config.FromEmail)
	fmt.Fprintf(&message, "To: %s\r\n", recipient)
	fmt.Fprintf(&message, "Subject: %s\r\n", encodedSubject)
	fmt.Fprintf(&message, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&message, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	fmt.Fprintf(&message, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n", boundary, normalizeSMTPBody(plainBody))
	fmt.Fprintf(&message, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n", boundary, normalizeSMTPBody(htmlBody))
	fmt.Fprintf(&message, "--%s--\r\n", boundary)
	return message.Bytes()
}

func normalizeSMTPBody(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}

func memoEmailBodies(memo model.Memo) (string, string) {
	plain := fmt.Sprintf("家庭备忘：%s\n提醒时间：%s", memo.Title, memo.RemindAt.Format("2006-01-02 15:04"))
	if memo.Description != "" {
		plain += "\n\n" + memo.Description
	}
	var html bytes.Buffer
	_ = template.Must(template.New("memo").Parse(`<div style="font-family:sans-serif;line-height:1.7;color:#3f2a20"><h2>🌻 {{.Title}}</h2><p><strong>提醒时间：</strong>{{.Time}}</p>{{if .Description}}<div style="padding:12px;background:#fff8df;border-radius:8px;white-space:pre-wrap">{{.Description}}</div>{{end}}<p style="color:#8a6b50">来自 VHome 家庭备忘</p></div>`)).Execute(&html, map[string]string{
		"Title": memo.Title, "Time": memo.RemindAt.Format("2006-01-02 15:04"), "Description": memo.Description,
	})
	return plain, html.String()
}
