package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/zellapay/zellapay/internal/collections"
)

func TestListPassesFiltersAndScopesToTheMerchant(t *testing.T) {
	f := &fakeService{}
	h := newHandler(f, "mer_1")
	rec := get(h.List, "/v1/collections?limit=10&starting_after=col_9"+
		"&status=pending&merchant_reference=O1")
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d: %s", rec.Code, rec.Body)
	}
	q := f.listed
	if q.MerchantID != "mer_1" || q.Limit != 10 || q.After != "col_9" || q.Reference != "O1" {
		t.Fatalf("service got %+v", q)
	}
	if len(q.Statuses) != 3 {
		t.Fatalf("pending must cover every unresolved state, got %v", q.Statuses)
	}
}

func TestListRejectsBadParameters(t *testing.T) {
	for _, target := range []string{
		"/v1/collections?limit=abc", "/v1/collections?status=bogus", "/v1/collections?limit=1.5",
	} {
		rec := get(newHandler(&fakeService{}, "mer_1").List, target)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: got %d", target, rec.Code)
		}
	}
}

func TestListResponseCarriesTheCursorOnlyWhenThereIsMore(t *testing.T) {
	more := collections.Page{
		Items:   []collections.Collection{sampleCollection()},
		HasMore: true, NextCursor: "col_1",
	}
	var got struct {
		Data       []map[string]any `json:"data"`
		HasMore    bool             `json:"has_more"`
		NextCursor *string          `json:"next_cursor"`
	}
	rec := get(newHandler(&fakeService{page: more}, "mer_1").List, "/v1/collections")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.HasMore || got.NextCursor == nil || *got.NextCursor != "col_1" || len(got.Data) != 1 {
		t.Fatalf("got %+v", got)
	}
	last := collections.Page{Items: []collections.Collection{sampleCollection()}}
	rec = get(newHandler(&fakeService{page: last}, "mer_1").List, "/v1/collections")
	got.NextCursor = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.HasMore || got.NextCursor != nil {
		t.Fatalf("last page must have no cursor: %+v", got)
	}
}

func TestListReturnsAnEmptyArrayNotNull(t *testing.T) {
	rec := get(newHandler(&fakeService{}, "mer_1").List, "/v1/collections")
	if !contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("got %s", rec.Body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
