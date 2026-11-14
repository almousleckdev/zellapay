package smtpsender

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/mail"
	"net/smtp"
	"time"

	"github.com/zellapay/zellapay/internal/notify"
	"github.com/zellapay/zellapay/internal/platform/clock"
)

const defaultTimeout = 20 * time.Second

// Config says how to reach the mail server.
type Config struct {
	Addr       string // host:port
	From       string // for example "ZellaPay <no-reply@zellapay.example>"
	Username   string
	Password   string
	RequireTLS bool          // refuse to send without STARTTLS; always true in production
	Hostname   string        // name announced to the server
	Timeout    time.Duration // for the whole conversation
}

// Sender implements notify.Sender over SMTP.
type Sender struct {
	cfg   Config
	from  mail.Address
	host  string
	clock clock.Clock
}

var _ notify.Sender = (*Sender)(nil)

// New validates cfg and returns a Sender.
func New(cfg Config, c clock.Clock) (*Sender, error) {
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil || host == "" {
		return nil, errors.New("smtpsender: Addr must be host:port")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, errors.New("smtpsender: From is not a valid address")
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "zellapay.local"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Sender{cfg: cfg, from: *from, host: host, clock: c}, nil
}

// Send delivers one message. Errors are *notify.SendError.
func (s *Sender) Send(ctx context.Context, d notify.Delivery) error {
	msg, err := build(envelope{
		From: s.from, To: d.To, Subject: d.Subject, Body: d.Body, ID: d.ID, Now: s.clock.Now(),
	})
	if err != nil {
		return &notify.SendError{Code: notify.CodeRejected, Permanent: true}
	}
	if err := s.deliver(ctx, d.To, msg); err != nil {
		return classify(err)
	}
	return nil
}

func (s *Sender) deliver(ctx context.Context, to string, msg []byte) error {
	conn, err := (&net.Dialer{Timeout: s.cfg.Timeout}).DialContext(ctx, "tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(s.deadline(ctx)); err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()
	if err := s.negotiate(c); err != nil {
		return err
	}
	return s.transmit(c, to, msg)
}

func (s *Sender) deadline(ctx context.Context) time.Time {
	d := time.Now().Add(s.cfg.Timeout)
	if cd, ok := ctx.Deadline(); ok && cd.Before(d) {
		return cd
	}
	return d
}

// negotiate greets the server, upgrades to TLS and authenticates.
func (s *Sender) negotiate(c *smtp.Client) error {
	if err := c.Hello(s.cfg.Hostname); err != nil {
		return err
	}
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(s.tlsConfig()); err != nil {
			return err
		}
	} else if s.cfg.RequireTLS {
		return errors.New("the server does not offer TLS")
	}
	if s.cfg.Username == "" {
		return nil
	}
	return c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.host))
}

func (s *Sender) tlsConfig() *tls.Config {
	return &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}
}

func (s *Sender) transmit(c *smtp.Client, to string, msg []byte) error {
	if err := c.Mail(s.from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
