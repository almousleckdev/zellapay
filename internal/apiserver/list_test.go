package apiserver_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func (e *env) ids(path string) (ids []string, hasMore bool, next string) {
	e.t.Helper()
	m := e.get(path, e.main.key).json(e.t)
	data, _ := m["data"].([]any)
	for _, item := range data {
		ids = append(ids, item.(map[string]any)["id"].(string))
	}
	hasMore, _ = m["has_more"].(bool)
	next, _ = m["next_cursor"].(string)
	return ids, hasMore, next
}

func TestPaginationWalksEveryCollectionOnceNewestFirst(t *testing.T) {
	e := newEnv(t)
	var created []string
	for i := 0; i < 5; i++ {
		e.clock.advance(time.Second)
		id, _ := e.create("00", fmt.Sprintf("idem-page-%d", i)).json(t)["id"].(string)
		created = append([]string{id}, created...) // newest first
	}
	var seen []string
	path := "/v1/collections?limit=2"
	for pages := 0; pages < 5; pages++ {
		got, more, next := e.ids(path)
		seen = append(seen, got...)
		if !more {
			break
		}
		path = "/v1/collections?limit=2&starting_after=" + next
	}
	if fmt.Sprint(seen) != fmt.Sprint(created) {
		t.Fatalf("walk = %v\nwant   %v", seen, created)
	}
}

func TestListFiltersByStatusAndReference(t *testing.T) {
	e := newEnv(t)
	e.create("00", "idem-f-1")
	e.create("01", "idem-f-2")
	e.clock.advance(3 * time.Second)
	e.deliverCallbacks()
	cases := map[string]int{
		"?status=succeeded": 1, "?status=failed": 1, "?status=pending": 0,
		"?merchant_reference=ORDER-01": 1, "?merchant_reference=nope": 0,
	}
	for query, want := range cases {
		got, _, _ := e.ids("/v1/collections" + query)
		if len(got) != want {
			t.Errorf("%s: %d items, want %d", query, len(got), want)
		}
	}
}

func TestListRejectsBadParameters(t *testing.T) {
	e := newEnv(t)
	bad := []string{"?limit=0x", "?limit=101", "?status=bogus", "?starting_after=col_nope"}
	for _, q := range bad {
		r := e.get("/v1/collections"+q, e.main.key)
		if r.status != http.StatusBadRequest || r.errorCode(t) != "invalid_request" {
			t.Errorf("%s: got %d %s", q, r.status, r.raw)
		}
	}
}

func TestACursorFromAnotherMerchantIsRefused(t *testing.T) {
	e := newEnv(t)
	id, _ := e.create("00", "idem-cur-1").json(t)["id"].(string)
	r := e.get("/v1/collections?starting_after="+id, e.other.key)
	if r.status != http.StatusBadRequest {
		t.Fatalf("got %d", r.status)
	}
}
