package logsender

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/zellapay/zellapay/internal/notify"
)

func TestTheCodeIsReadableButTheNumberIsMasked(t *testing.T) {
	var buf bytes.Buffer
	s := New(slog.New(slog.NewTextHandler(&buf, nil)))
	err := s.Send(context.Background(), notify.Delivery{
		ID: "ntf_1", To: "+24107123456", Body: "ZellaPay : votre code est 482913",
	})
	if err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "482913") {
		t.Fatalf("a developer must be able to read the code: %s", out)
	}
	if strings.Contains(out, "+24107123456") || !strings.Contains(out, "+241 07 ** ** 56") {
		t.Fatalf("the number must be masked: %s", out)
	}
	if !strings.Contains(out, "not sent") {
		t.Fatalf("the line must say nothing was sent: %s", out)
	}
}
