package api_test

import (
	"sort"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

type operation struct {
	name   string
	method string
	op     *v3.Operation
}

// operations lists every path operation in a stable order.
func operations(t *testing.T) []operation {
	t.Helper()
	var out []operation
	paths := loadModel(t).Paths.PathItems
	for path, item := range paths.FromOldest() {
		for method, op := range item.GetOperations().FromOldest() {
			out = append(out, operation{name: method + " " + path, method: method, op: op})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	if len(out) == 0 {
		t.Fatal("no operations found")
	}
	return out
}
