// Package maildevtest starts a disposable MailDev for integration tests and
// reads back what it received. It must only be imported from tests.
package maildevtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Server is a running MailDev.
type Server struct {
	SMTPAddr string // host:port to send to
	api      string
	stop     func()
}

// Mail is a message MailDev received.
type Mail struct {
	Subject string
	Text    string
	To      string
	From    string
}

// Start launches MailDev and returns it. Call Stop when done.
func Start() (*Server, error) {
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "maildev/maildev:2.1.0",
			ExposedPorts: []string{"1025/tcp", "1080/tcp"},
			WaitingFor:   wait.ForListeningPort("1080/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start maildev: %w", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, err
	}
	smtp, err := c.MappedPort(ctx, "1025/tcp")
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, err
	}
	web, err := c.MappedPort(ctx, "1080/tcp")
	if err != nil {
		_ = c.Terminate(ctx)
		return nil, err
	}
	return &Server{
		SMTPAddr: net.JoinHostPort(host, smtp.Port()),
		api:      "http://" + net.JoinHostPort(host, web.Port()),
		stop:     func() { _ = c.Terminate(ctx) },
	}, nil
}

// Stop removes the container.
func (s *Server) Stop() { s.stop() }

// Messages returns everything received so far.
func (s *Server) Messages() ([]Mail, error) {
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodGet, s.api+"/email", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var raw []struct {
		Subject string                     `json:"subject"`
		Text    string                     `json:"text"`
		To      []struct{ Address string } `json:"to"`
		From    []struct{ Address string } `json:"from"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	out := make([]Mail, len(raw))
	for i, m := range raw {
		out[i] = Mail{Subject: m.Subject, Text: m.Text}
		if len(m.To) > 0 {
			out[i].To = m.To[0].Address
		}
		if len(m.From) > 0 {
			out[i].From = m.From[0].Address
		}
	}
	return out, nil
}

// Clear deletes everything received so far.
func (s *Server) Clear() error {
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodDelete, s.api+"/email/all", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}
