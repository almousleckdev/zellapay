package smtpsender

import (
	"errors"
	"mime"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func envelopeFor(subject, body string) envelope {
	return envelope{
		From: mail.Address{Name: "ZellaPay", Address: "no-reply@zellapay.test"},
		To:   "owner@example.com", Subject: subject, Body: body, ID: "ntf_123",
		Now: time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC),
	}
}

func TestAMessageHasTheRequiredHeadersAndCRLFLines(t *testing.T) {
	raw, err := build(envelopeFor("Hello", "Line one\nLine two"))
	if err != nil {
		t.Fatal(err)
	}
	msg := string(raw)
	for _, want := range []string{
		"From: \"ZellaPay\" <no-reply@zellapay.test>\r\n", "To: owner@example.com\r\n",
		"Subject: Hello\r\n", "Date: Sat, 14 Nov 2026 12:00:00 +0000\r\n",
		"Message-ID: <ntf_123@zellapay.test>\r\n", "MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n", "Auto-Submitted: auto-generated\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in:\n%s", want, msg)
		}
	}
	if strings.Contains(strings.ReplaceAll(msg, "\r\n", ""), "\n") {
		t.Error("every line break must be CRLF")
	}
}

func TestNonASCIISubjectsAreEncoded(t *testing.T) {
	raw, err := build(envelopeFor("Invitation à rejoindre Boutique Océane", "x"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(parsed.Header.Get("Subject"), "àÔé") {
		t.Fatal("the raw subject header must be ASCII")
	}
	got, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || got != "Invitation à rejoindre Boutique Océane" {
		t.Fatalf("decoded %q, %v", got, err)
	}
}

func TestControlCharactersInHeadersAreRefused(t *testing.T) {
	withTo, withID := envelopeFor("Hi", "x"), envelopeFor("Hi", "x")
	withTo.To = "a@b.com\r\nBcc: x@y.z"
	withID.ID = "a\nb"
	cases := map[string]envelope{
		"subject": envelopeFor("Hi\r\nBcc: x@y.z", "x"),
		"to":      withTo,
		"id":      withID,
	}
	for name, e := range cases {
		if _, err := build(e); !errors.Is(err, errHeaderInjection) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestNewValidatesItsConfiguration(t *testing.T) {
	for name, cfg := range map[string]Config{
		"no port":  {Addr: "localhost", From: "a@b.com"},
		"no host":  {Addr: ":25", From: "a@b.com"},
		"bad from": {Addr: "localhost:25", From: "not an address"},
	} {
		if _, err := New(cfg, nil); err == nil {
			t.Errorf("%s must be rejected", name)
		}
	}
}
