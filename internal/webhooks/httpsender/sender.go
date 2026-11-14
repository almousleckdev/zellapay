// Package httpsender delivers webhooks over HTTP without letting merchant
// supplied URLs reach internal networks.
package httpsender

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"

	"github.com/zellapay/zellapay/internal/webhooks"
)

const (
	dialTimeout  = 5 * time.Second
	drainLimit   = 4 << 10
	maxHeaderLen = 16 << 10
)

var errBlocked = errors.New("destination address is not allowed")

// Sender implements webhooks.Sender.
type Sender struct {
	client *http.Client
}

var _ webhooks.Sender = (*Sender)(nil)

// New returns a Sender that enforces policy on every connection and gives up
// after timeout.
func New(policy webhooks.Policy, timeout time.Duration) *Sender {
	dialer := &net.Dialer{Timeout: dialTimeout, Control: guard(policy)}
	transport := &http.Transport{
		Proxy:                  nil, // never route through an environment proxy
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    dialTimeout,
		ResponseHeaderTimeout:  timeout,
		DisableKeepAlives:      true,
		MaxResponseHeaderBytes: maxHeaderLen,
	}
	return &Sender{client: &http.Client{
		Transport: transport,
		Timeout:   timeout,
		// A redirect could point at an internal address. Never follow one.
		CheckRedirect: neverRedirect,
	}}
}

func neverRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

// guard rejects a connection to a forbidden address. It runs after name
// resolution, on the exact address about to be dialled, so DNS tricks such
// as rebinding cannot bypass it.
func guard(p webhooks.Policy) func(network, address string, c syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return errBlocked
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !p.AllowedAddr(ip) {
			return errBlocked
		}
		return nil
	}
}

// Send posts the request and returns the response status.
func (s *Sender) Send(ctx context.Context, r webhooks.SendRequest) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.URL, bytes.NewReader(r.Body))
	if err != nil {
		return 0, &webhooks.SendError{Code: webhooks.CodeUnknown}
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, &webhooks.SendError{Code: classify(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit))
	return resp.StatusCode, nil
}

// classify reduces a transport error to a stable code. Raw errors can
// contain internal addresses and are never stored or shown.
func classify(err error) string {
	var (
		unknownCA x509.UnknownAuthorityError
		hostErr   x509.HostnameError
		certErr   x509.CertificateInvalidError
		recErr    tls.RecordHeaderError
		netErr    net.Error
	)
	switch {
	case errors.Is(err, errBlocked):
		return webhooks.CodeBlockedHost
	case errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()):
		return webhooks.CodeTimeout
	case errors.As(err, &unknownCA), errors.As(err, &hostErr),
		errors.As(err, &certErr), errors.As(err, &recErr):
		return webhooks.CodeTLS
	}
	return webhooks.CodeConnection
}
