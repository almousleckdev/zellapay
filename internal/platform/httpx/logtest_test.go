package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

type logSink struct {
	buf bytes.Buffer
}

func newLogSink() (*slog.Logger, *logSink) {
	s := &logSink{}
	return slog.New(slog.NewJSONHandler(&s.buf, nil)), s
}

func (s *logSink) records(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	dec := json.NewDecoder(bytes.NewReader(s.buf.Bytes()))
	for dec.More() {
		var m map[string]any
		if err := dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		out = append(out, m)
	}
	return out
}

func (s *logSink) find(t *testing.T, msg string) map[string]any {
	t.Helper()
	for _, m := range s.records(t) {
		if m["msg"] == msg {
			return m
		}
	}
	t.Fatalf("no log record %q in %s", msg, s.buf.String())
	return nil
}
