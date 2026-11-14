package simulator

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zellapay/zellapay/internal/operators"
)

const (
	// SignatureHeader carries the hex HMAC-SHA256 of the request body.
	SignatureHeader = "X-Sim-Signature"
	maxCallbackBody = 64 << 10
)

// Callback is a signed notification the simulated operator would POST to
// the gateway. A development harness delivers it to the callback endpoint.
type Callback struct {
	Body      []byte
	Signature string
}

type callbackBody struct {
	Reference string `json:"reference"`
	OpRef     string `json:"operator_ref"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

// DueCallbacks returns the callbacks that have become due, each at most
// as many times as its scenario prescribes. Calling it again returns only
// callbacks not yet handed out.
func (s *Simulator) DueCallbacks() []Callback {
	s.mu.Lock()
	defer s.mu.Unlock()
	var due []Callback
	for _, t := range s.txns {
		o := t.outcomeAt(s)
		if o.state == operators.StatePending {
			continue
		}
		for ; t.sent < t.scenario.callbackCount(); t.sent++ {
			due = append(due, s.sign(t.callbackBody(o)))
		}
	}
	return due
}

func (t *txn) callbackBody(o outcome) callbackBody {
	amount := t.amount
	if t.scenario == scenWrongAmount {
		amount++
	}
	return callbackBody{
		Reference: t.reference, OpRef: t.opRef, Status: o.code,
		Amount: amount, Currency: t.currency,
	}
}

func (s *Simulator) sign(b callbackBody) Callback {
	raw, _ := json.Marshal(b)
	return Callback{Body: raw, Signature: s.mac(raw)}
}

func (s *Simulator) mac(body []byte) string {
	m := hmac.New(sha256.New, s.cfg.Secret)
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// ParseCallback verifies the signature and decodes the callback. The
// caller must still compare the reported amount with the request.
func (s *Simulator) ParseCallback(
	_ context.Context, req *http.Request,
) (operators.CallbackEvent, error) {
	raw, err := io.ReadAll(io.LimitReader(req.Body, maxCallbackBody))
	if err != nil {
		return operators.CallbackEvent{}, invalid("read body")
	}
	got, err := hex.DecodeString(req.Header.Get(SignatureHeader))
	want, _ := hex.DecodeString(s.mac(raw))
	if err != nil || !hmac.Equal(got, want) {
		return operators.CallbackEvent{}, invalid("bad signature")
	}
	return decode(raw)
}

func invalid(reason string) error {
	return fmt.Errorf("%w: %s", operators.ErrInvalidCallback, reason)
}

func decode(raw []byte) (operators.CallbackEvent, error) {
	var b callbackBody
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil || b.Reference == "" {
		return operators.CallbackEvent{}, invalid("malformed body")
	}
	return operators.CallbackEvent{
		Ref:      operators.Ref{OperatorRef: b.OpRef, OurRef: b.Reference},
		Status:   operators.Status{State: stateOf(b.Status), RawCode: b.Status, RawBody: raw},
		Amount:   b.Amount,
		Currency: b.Currency,
	}, nil
}

func stateOf(code string) operators.State {
	switch code {
	case "SUCCESS":
		return operators.StateSucceeded
	case "PENDING":
		return operators.StatePending
	case "TIMEOUT":
		return operators.StateExpired
	case "":
		return operators.StateUnknown
	}
	return operators.StateFailed
}
