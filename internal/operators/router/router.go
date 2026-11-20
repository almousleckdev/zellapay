// Package router chooses the operator adapter for a phone number.
package router

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/zellapay/zellapay/internal/operators"
)

// ErrNoRoute means no rule matches the phone number.
var ErrNoRoute = errors.New("router: no operator for this number")

// Rule sends numbers starting with Prefix to the named operator. Rules are
// configuration, not code: numbering plans change.
type Rule struct {
	Prefix   string // E.164 prefix such as "+24107"
	Operator string // adapter code
}

// Router maps phone numbers to adapters by longest matching prefix.
type Router struct {
	rules    []Rule
	adapters map[string]operators.Adapter
}

// New builds a Router. Every rule must name a provided adapter.
func New(rules []Rule, adapters ...operators.Adapter) (*Router, error) {
	byCode := make(map[string]operators.Adapter, len(adapters))
	for _, a := range adapters {
		byCode[a.Code()] = a
	}
	for _, r := range rules {
		if !strings.HasPrefix(r.Prefix, "+") || len(r.Prefix) < 3 {
			return nil, fmt.Errorf("router: invalid prefix %q", r.Prefix)
		}
		if _, ok := byCode[r.Operator]; !ok {
			return nil, fmt.Errorf("router: rule %q names unknown operator %q",
				r.Prefix, r.Operator)
		}
	}
	sorted := append([]Rule(nil), rules...)
	sort.Slice(sorted, func(i, j int) bool { return len(sorted[i].Prefix) > len(sorted[j].Prefix) })
	return &Router{rules: sorted, adapters: byCode}, nil
}

// Rules returns the routing rules, longest prefix first.
func (r *Router) Rules() []Rule { return append([]Rule(nil), r.rules...) }

// Route returns the adapter for phone, preferring the longest prefix.
func (r *Router) Route(phone string) (operators.Adapter, error) {
	for _, rule := range r.rules {
		if strings.HasPrefix(phone, rule.Prefix) {
			return r.adapters[rule.Operator], nil
		}
	}
	return nil, ErrNoRoute
}

// Adapter returns the adapter registered under code.
func (r *Router) Adapter(code string) (operators.Adapter, bool) {
	a, ok := r.adapters[code]
	return a, ok
}
