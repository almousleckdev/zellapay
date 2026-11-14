package api_test

import "testing"

// Project rules the contract must obey. See CLAUDE.md, "API and contracts".

func TestEveryOperationIsDocumented(t *testing.T) {
	for _, op := range operations(t) {
		if op.op.OperationId == "" || len(op.op.Tags) == 0 || op.op.Summary == "" {
			t.Errorf("%s: needs operationId, tags and summary", op.name)
		}
	}
}

func TestEveryPostRequiresAnIdempotencyKey(t *testing.T) {
	for _, op := range operations(t) {
		if op.method != "post" {
			continue
		}
		found := false
		for _, p := range op.op.Parameters {
			if p.Name == "Idempotency-Key" && p.Required != nil && *p.Required {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: POST without a required Idempotency-Key", op.name)
		}
	}
}

func TestEveryOperationDocumentsAuthenticationFailure(t *testing.T) {
	for _, op := range operations(t) {
		if op.op.Responses == nil || op.op.Responses.Codes.GetOrZero("401") == nil {
			t.Errorf("%s: missing 401 response", op.name)
		}
	}
}

func TestOperationIDsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, op := range operations(t) {
		if prev, dup := seen[op.op.OperationId]; dup {
			t.Errorf("%s duplicates operationId of %s", op.name, prev)
		}
		seen[op.op.OperationId] = op.name
	}
}
