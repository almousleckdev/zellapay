package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func invite() Message {
	return Message{
		To: "owner@example.com", Template: "invitation", Locale: "fr",
		Data: sample("invitation"),
	}
}

func (h *harness) enqueue(t *testing.T, m Message) string {
	t.Helper()
	id, stored, err := h.svc.Enqueue(context.Background(), m)
	if err != nil || !stored {
		t.Fatalf("enqueue: %v stored=%v", err, stored)
	}
	return id
}

func (h *harness) deliver(t *testing.T) DeliverResult {
	t.Helper()
	res, err := h.svc.DeliverDue(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (h *harness) record(t *testing.T, id string) Record {
	t.Helper()
	r, err := h.repo.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAMessageIsRenderedAndSent(t *testing.T) {
	h := newHarness(t)
	id := h.enqueue(t, invite())
	if res := h.deliver(t); res.Sent != 1 {
		t.Fatalf("got %+v", res)
	}
	got := h.email.sent[0]
	if got.To != "owner@example.com" || !strings.Contains(got.Subject, "Boutique Océane") ||
		!strings.Contains(got.Body, "https://app.example/invite/abc") {
		t.Fatalf("delivery %+v", got)
	}
	if rec := h.record(t, id); rec.Status != StatusSent || rec.SentAt == nil {
		t.Fatalf("record %+v", rec)
	}
}

func TestSMSGoesToTheSMSSender(t *testing.T) {
	h := newHarness(t)
	h.enqueue(t, Message{To: "+24107123456", Template: "security_alert", Locale: "en",
		Data: sample("security_alert")})
	h.deliver(t)
	if h.sms.count() != 1 || h.email.count() != 0 {
		t.Fatalf("sms %d, email %d", h.sms.count(), h.email.count())
	}
}

func TestThePayloadIsSealedAndErasedOnceSent(t *testing.T) {
	h := newHarness(t)
	id := h.enqueue(t, invite())
	sealed := h.repo.rows[id].p.PayloadSealed
	secrets := []string{"owner@example.com", "https://app.example/invite/abc", "Océane"}
	for _, secret := range secrets {
		if strings.Contains(sealed, secret) {
			t.Fatalf("the stored payload reveals %q", secret)
		}
	}
	h.deliver(t)
	if got := h.repo.rows[id].p.PayloadSealed; got != "" || !h.record(t, id).PayloadErased {
		t.Fatalf("a sent message must not keep its link or address, got %q", got)
	}
}

func TestFailuresAreRetriedWithBackoffThenSucceed(t *testing.T) {
	h := newHarness(t)
	h.email.failures = []error{&SendError{Code: CodeUnavailable}, &SendError{Code: CodeTimeout}}
	id := h.enqueue(t, invite())

	if res := h.deliver(t); res.Retrying != 1 {
		t.Fatalf("first: %+v", res)
	}
	if h.deliver(t).Attempted != 0 {
		t.Fatal("not due yet")
	}
	h.clock.advance(Backoff(1))
	h.deliver(t)
	h.clock.advance(Backoff(2) - time.Second)
	if h.deliver(t).Attempted != 0 {
		t.Fatal("one second early must not retry")
	}
	h.clock.advance(time.Second)
	if res := h.deliver(t); res.Sent != 1 {
		t.Fatalf("third attempt: %+v", res)
	}
	if rec := h.record(t, id); rec.Attempts != 3 || rec.Status != StatusSent {
		t.Fatalf("record %+v", rec)
	}
}

func TestARefusedRecipientIsNotRetried(t *testing.T) {
	h := newHarness(t)
	h.email.failures = []error{&SendError{Code: CodeRejected, Permanent: true}}
	id := h.enqueue(t, invite())
	if res := h.deliver(t); res.Dead != 1 {
		t.Fatalf("got %+v", res)
	}
	h.clock.advance(48 * time.Hour)
	if h.deliver(t).Attempted != 0 || h.email.count() != 1 {
		t.Fatal("a dead message must never be sent again")
	}
	rec := h.record(t, id)
	if rec.Status != StatusDead || rec.LastError != CodeRejected || !rec.PayloadErased {
		t.Fatalf("record %+v", rec)
	}
}

func TestGivingUpAfterTheRetryWindow(t *testing.T) {
	h := newHarness(t)
	h.email.failures = []error{&SendError{Code: CodeUnavailable}}
	h.email.failures = append(h.email.failures, make([]error, 200)...)
	for i := range h.email.failures[1:] {
		h.email.failures[i+1] = &SendError{Code: CodeUnavailable}
	}
	id := h.enqueue(t, invite())
	start := h.clock.Now()
	for i := 0; i < 100 && h.record(t, id).Status == StatusPending; i++ {
		h.deliver(t)
		h.clock.advance(Backoff(i + 1))
	}
	if rec := h.record(t, id); rec.Status != StatusDead {
		t.Fatalf("record %+v", rec)
	}
	if span := h.clock.Now().Sub(start); span > RetryHorizon+2*time.Hour {
		t.Fatalf("kept retrying for %v", span)
	}
}

func TestAnUnknownFailureIsRetriedAsTemporary(t *testing.T) {
	h := newHarness(t)
	h.email.failures = []error{errors.New("something odd")}
	id := h.enqueue(t, invite())
	if res := h.deliver(t); res.Retrying != 1 {
		t.Fatalf("got %+v", res)
	}
	if h.record(t, id).LastError != CodeUnknown {
		t.Fatal("raw error text must never be stored")
	}
}

func TestADuplicateDedupeKeyIsIgnored(t *testing.T) {
	h := newHarness(t)
	m := invite()
	m.DedupeKey = "invite:inv_1"
	h.enqueue(t, m)
	if _, stored, err := h.svc.Enqueue(context.Background(), m); err != nil || stored {
		t.Fatalf("a repeat must be ignored: stored=%v err=%v", stored, err)
	}
	h.deliver(t)
	if h.email.count() != 1 {
		t.Fatalf("sent %d times", h.email.count())
	}
}

func TestBadMessagesAreRefusedBeforeStoring(t *testing.T) {
	h := newHarness(t)
	mutate := func(f func(*Message)) Message { m := invite(); f(&m); return m }
	sms := sample("security_alert")
	cases := map[string]Message{
		"unknown template": mutate(func(m *Message) { m.Template = "nope" }),
		"bad locale":       mutate(func(m *Message) { m.Locale = "de" }),
		"header injection": mutate(func(m *Message) { m.To = "a@b.com\r\nBcc: x@y.z" }),
		"line break value": mutate(func(m *Message) { m.Data["business"] = "Shop\nBcc: x@y.z" }),
		"missing data":     mutate(func(m *Message) { delete(m.Data, "link") }),
		"sms to an email":  {To: "a@b.com", Template: "security_alert", Data: sms},
	}
	for name, m := range cases {
		if _, _, err := h.svc.Enqueue(context.Background(), m); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if len(h.repo.rows) != 0 {
		t.Fatal("nothing may be stored for a refused message")
	}
}

func TestAChannelWithNoSenderIsRefused(t *testing.T) {
	h := newHarness(t)
	delete(h.svc.d.Senders, SMS)
	_, _, err := h.svc.Enqueue(context.Background(), Message{
		To: "+24107123456", Template: "security_alert", Data: sample("security_alert"),
	})
	if !errors.Is(err, ErrChannelUnavailable) {
		t.Fatalf("got %v", err)
	}
}

func TestFrenchIsTheDefaultLanguage(t *testing.T) {
	h := newHarness(t)
	m := invite()
	m.Locale = ""
	h.enqueue(t, m)
	h.deliver(t)
	if !strings.Contains(h.email.sent[0].Body, "Bonjour") {
		t.Fatalf("body %q", h.email.sent[0].Body)
	}
}
