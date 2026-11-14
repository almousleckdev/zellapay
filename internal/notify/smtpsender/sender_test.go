package smtpsender

import (
	"context"
	"errors"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/notify"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC) }

const (
	frenchSubject = "Invitation à rejoindre Boutique Océane sur ZellaPay"
	inviteLink    = "https://app.example/invite/abc?x=1&y=2"
)

// hi is the smallest valid delivery to a recipient.
func hi(id, to string) notify.Delivery {
	return notify.Delivery{ID: id, To: to, Subject: "Hi", Body: "x"}
}

func newSender(t *testing.T, mutate func(*Config)) *Sender {
	t.Helper()
	cfg := Config{
		Addr: maildev.SMTPAddr, From: "ZellaPay <no-reply@zellapay.test>", Timeout: 5 * time.Second,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	s, err := New(cfg, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func code(t *testing.T, err error) *notify.SendError {
	t.Helper()
	var se *notify.SendError
	if !errors.As(err, &se) {
		t.Fatalf("want a SendError, got %v", err)
	}
	return se
}

func TestAFrenchEmailArrivesIntactInMailDev(t *testing.T) {
	_ = maildev.Clear()
	s := newSender(t, nil)
	err := s.Send(context.Background(), notify.Delivery{
		ID: "ntf_1", To: "owner@example.com",
		Subject: frenchSubject,
		Body:    "Bonjour,\n\nVous avez été invité(e).\n" + inviteLink + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	mails, err := maildev.Messages()
	if err != nil || len(mails) != 1 {
		t.Fatalf("%d messages, %v", len(mails), err)
	}
	m := mails[0]
	rightHeaders := m.Subject == frenchSubject && m.To == "owner@example.com" &&
		m.From == "no-reply@zellapay.test"
	if !rightHeaders {
		t.Fatalf("got %+v", m)
	}
	for _, want := range []string{"invité(e)", inviteLink} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("the body lost %q: %q", want, m.Text)
		}
	}
}

func TestEveryRecipientGetsItsOwnMessage(t *testing.T) {
	_ = maildev.Clear()
	s := newSender(t, nil)
	for _, to := range []string{"a@example.com", "b@example.com", "c@example.com"} {
		if err := s.Send(context.Background(), hi("ntf_x", to)); err != nil {
			t.Fatal(err)
		}
	}
	mails, _ := maildev.Messages()
	if len(mails) != 3 {
		t.Fatalf("got %d messages", len(mails))
	}
}

func TestRequiringTLSRefusesAServerWithoutIt(t *testing.T) {
	_ = maildev.Clear()
	s := newSender(t, func(c *Config) { c.RequireTLS = true })
	err := s.Send(context.Background(), hi("ntf_t", "a@example.com"))
	if se := code(t, err); se.Permanent || se.Code != notify.CodeUnavailable {
		t.Fatalf("got %+v: it must be retried and noticed, not discarded", se)
	}
	if mails, _ := maildev.Messages(); len(mails) != 0 {
		t.Fatal("nothing may be sent over an unencrypted connection when TLS is required")
	}
}

func TestAnUnreachableServerIsATemporaryFailure(t *testing.T) {
	l, _ := listen()
	addr := l.Addr().String()
	_ = l.Close()
	s := newSender(t, func(c *Config) { c.Addr = addr })
	err := s.Send(context.Background(), hi("ntf_u", "a@example.com"))
	if se := code(t, err); se.Permanent || se.Code != notify.CodeUnavailable {
		t.Fatalf("got %+v", se)
	}
	if strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("the error leaks an address: %v", err)
	}
}

func TestASilentServerTimesOut(t *testing.T) {
	l, err := listen()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			defer func() { _ = c.Close() }() // accepts and says nothing
		}
	}()
	s := newSender(t, func(c *Config) {
		c.Addr, c.Timeout = l.Addr().String(), 300*time.Millisecond
	})
	start := time.Now()
	sendErr := s.Send(context.Background(), hi("ntf_s", "a@example.com"))
	if se := code(t, sendErr); se.Code != notify.CodeTimeout || time.Since(start) > 3*time.Second {
		t.Fatalf("got %+v after %v", se, time.Since(start))
	}
}

func TestInjectedHeadersNeverReachTheWire(t *testing.T) {
	_ = maildev.Clear()
	s := newSender(t, nil)
	err := s.Send(context.Background(), notify.Delivery{
		ID: "ntf_i", To: "a@example.com\r\nBcc: attacker@example.com", Subject: "Hi", Body: "x",
	})
	if se := code(t, err); !se.Permanent {
		t.Fatalf("got %+v", se)
	}
	if mails, _ := maildev.Messages(); len(mails) != 0 {
		t.Fatal("a message with an injected header must not be sent")
	}
}

func TestSMTPRepliesAreClassifiedByTheirCode(t *testing.T) {
	cases := []struct {
		code      int
		permanent bool
		want      string
	}{
		{550, true, notify.CodeRejected}, {553, true, notify.CodeRejected},
		{450, false, notify.CodeUnavailable}, {421, false, notify.CodeUnavailable},
		{535, false, notify.CodeUnavailable}, {530, false, notify.CodeUnavailable},
	}
	for _, c := range cases {
		got := classify(&textproto.Error{Code: c.code, Msg: "secret server banner"})
		if got.Permanent != c.permanent || got.Code != c.want {
			t.Errorf("%d: got %+v", c.code, got)
		}
		if strings.Contains(got.Error(), "banner") {
			t.Errorf("%d: the server text must not be kept", c.code)
		}
	}
}

// listen opens a local port that nothing else uses.
func listen() (net.Listener, error) {
	return (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
}
