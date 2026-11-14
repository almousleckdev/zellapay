package simulator

import (
	"context"
	"encoding/json"

	"github.com/zellapay/zellapay/internal/operators"
)

// outcome is the transaction's state and operator code right now.
type outcome struct {
	state operators.State
	code  string
}

func (t *txn) outcomeAt(s *Simulator) outcome {
	if s.clock.Now().Sub(t.created) < s.cfg.Delay {
		return outcome{operators.StatePending, "PENDING"}
	}
	switch t.scenario {
	case scenExpires:
		return outcome{operators.StateExpired, "TIMEOUT"}
	case scenRejected:
		return outcome{operators.StateFailed, "REJECTED"}
	case scenDeclined:
		return outcome{operators.StateFailed, "CUSTOMER_DECLINED"}
	case scenInsufficientFunds:
		return outcome{operators.StateFailed, "INSUFFICIENT_FUNDS"}
	case scenNeverCompletes:
		return outcome{operators.StatePending, "PENDING"}
	}
	return outcome{operators.StateSucceeded, "SUCCESS"}
}

// Status returns the transaction's current state by our reference. An
// unknown reference means the operator never received the request.
func (s *Simulator) Status(_ context.Context, ref operators.Ref) (operators.Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.txns[ref.OurRef]
	if !ok {
		return operators.Status{}, operators.ErrNotFound
	}
	o := t.outcomeAt(s)
	body, err := json.Marshal(map[string]any{
		"reference": t.reference, "status": o.code, "amount": t.amount,
	})
	if err != nil {
		return operators.Status{}, err
	}
	return operators.Status{State: o.state, RawCode: o.code, RawBody: body}, nil
}
