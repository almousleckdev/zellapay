// Package smtpsender delivers email over SMTP using only the standard
// library. In development it talks to MailDev; in production it requires TLS.
package smtpsender

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
	"unicode"
)

// errHeaderInjection means a header value contained a line break or another
// control character. Recipients are validated earlier; this is the last line
// of defence for the one place where headers are actually written.
var errHeaderInjection = errors.New("smtpsender: control character in a header")

type envelope struct {
	From    mail.Address
	To      string
	Subject string
	Body    string
	ID      string
	Now     time.Time
}

// build renders the full RFC 5322 message with CRLF line endings.
func build(e envelope) ([]byte, error) {
	for _, v := range []string{e.From.Address, e.From.Name, e.To, e.Subject, e.ID} {
		if strings.IndexFunc(v, unicode.IsControl) >= 0 {
			return nil, errHeaderInjection
		}
	}
	domain := e.From.Address[strings.LastIndex(e.From.Address, "@")+1:]
	var b bytes.Buffer
	header := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	header("From", e.From.String())
	header("To", e.To)
	header("Subject", mime.QEncoding.Encode("utf-8", e.Subject))
	header("Date", e.Now.UTC().Format(time.RFC1123Z))
	header("Message-ID", "<"+e.ID+"@"+domain+">")
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=UTF-8")
	header("Content-Transfer-Encoding", "quoted-printable")
	header("Auto-Submitted", "auto-generated")
	b.WriteString("\r\n")
	w := quotedprintable.NewWriter(&b)
	if _, err := w.Write([]byte(e.Body)); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
