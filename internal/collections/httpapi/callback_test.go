package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zellapay/zellapay/internal/collections"
	"github.com/zellapay/zellapay/internal/operators"
)

func TestCallbackStatusMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, http.StatusNoContent},
		{collections.ErrUnknownReference, http.StatusNoContent},
		{collections.ErrUnknownOperator, http.StatusNotFound},
		{operators.ErrInvalidCallback, http.StatusBadRequest},
		{errors.New("database down"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		f := &fakeService{err: c.err}
		mux := http.NewServeMux()
		mux.HandleFunc("POST /callbacks/{operator}", newHandler(f, "").Callback)
		req := httptest.NewRequestWithContext(
			context.Background(), "POST", "/callbacks/simulator", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%v: got %d, want %d", c.err, rec.Code, c.want)
		}
		if f.callback != "simulator" {
			t.Errorf("operator = %q", f.callback)
		}
	}
}
