package httpx

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover turns a handler panic into a logged, sanitized 500 response. It
// writes nothing once the response has started, and lets
// http.ErrAbortHandler through.
func Recover(log *slog.Logger) Decorator {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := asStatusRecorder(w)
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				reportPanic(log, rec, r, v)
			}()
			next.ServeHTTP(rec, r)
		})
	}
}

func reportPanic(log *slog.Logger, rec *statusRecorder, r *http.Request, v any) {
	log.Error("panic recovered",
		"request_id", RequestID(r.Context()),
		"method", r.Method,
		"path", r.URL.Path,
		"panic", fmt.Sprint(v),
		"stack", string(debug.Stack()),
		"response_committed", rec.wrote,
	)
	if !rec.wrote {
		WriteError(rec, r, problemInternal)
	}
}
