package simulator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zellapay/zellapay/internal/operators"
)

func TestScenarioSelectionByPhoneSuffix(t *testing.T) {
	cases := map[string]scenario{
		"00": scenSucceed, "01": scenDeclined, "02": scenInsufficientFunds,
		"03": scenTimeoutThenOK, "04": scenNeverCompletes, "05": scenDuplicateCallback,
		"06": scenMissingCallback, "07": scenWrongAmount, "08": scenRejected,
		"09": scenExpires, "99": scenSucceed,
	}
	for suffix, want := range cases {
		if got := scenarioFor(phone(suffix)); got != want {
			t.Errorf("suffix %s: got %v, want %v", suffix, got, want)
		}
	}
	if scenarioFor("") != scenSucceed {
		t.Error("empty phone defaults to success")
	}
}

func TestSuccessPendingThenSucceeded(t *testing.T) {
	s, clk := newSim(t)
	ref, err := collect(t, s, "00")
	if err != nil {
		t.Fatal(err)
	}
	if statusOf(t, s, ref) != operators.StatePending {
		t.Fatal("must start pending")
	}
	clk.advance(6 * time.Second)
	if statusOf(t, s, ref) != operators.StateSucceeded {
		t.Fatal("must succeed after the delay")
	}
}

func TestFailureScenariosCarryAReasonCode(t *testing.T) {
	codes := map[string]string{"01": "CUSTOMER_DECLINED", "02": "INSUFFICIENT_FUNDS"}
	for suffix, code := range codes {
		s, clk := newSim(t)
		ref, _ := collect(t, s, suffix)
		clk.advance(6 * time.Second)
		st, err := s.Status(context.Background(), ref)
		if err != nil || st.State != operators.StateFailed || st.RawCode != code {
			t.Errorf("%s: %+v %v", suffix, st, err)
		}
	}
}

func TestTimeoutScenarioHidesAnOutcomeThatStillHappens(t *testing.T) {
	s, clk := newSim(t)
	ref, err := collect(t, s, "03")
	if !errors.Is(err, operators.ErrUnknownOutcome) {
		t.Fatalf("initiation must time out, got %v", err)
	}
	clk.advance(6 * time.Second)
	if statusOf(t, s, ref) != operators.StateSucceeded {
		t.Fatal("the operator processed it; status must reveal that")
	}
}

func TestNeverCompletingStaysPendingWithoutCallbacks(t *testing.T) {
	s, clk := newSim(t)
	ref, _ := collect(t, s, "04")
	clk.advance(24 * time.Hour)
	if statusOf(t, s, ref) != operators.StatePending || len(s.DueCallbacks()) != 0 {
		t.Fatal("must stay pending and silent")
	}
}

func TestCallbackCountsPerScenario(t *testing.T) {
	for suffix, want := range map[string]int{"00": 1, "05": 2, "06": 0, "07": 1} {
		s, clk := newSim(t)
		_, _ = collect(t, s, suffix)
		if got := len(s.DueCallbacks()); got != 0 {
			t.Errorf("%s: callbacks before the delay: %d", suffix, got)
		}
		clk.advance(6 * time.Second)
		if got := len(s.DueCallbacks()); got != want {
			t.Errorf("%s: got %d callbacks, want %d", suffix, got, want)
		}
		if got := len(s.DueCallbacks()); got != 0 {
			t.Errorf("%s: callbacks must not repeat, got %d", suffix, got)
		}
	}
}

func TestWrongAmountScenarioReportsADifferentAmount(t *testing.T) {
	s, clk := newSim(t)
	_, _ = collect(t, s, "07")
	clk.advance(6 * time.Second)
	cb := s.DueCallbacks()[0]
	ev, err := s.ParseCallback(context.Background(), callbackRequest(cb))
	if err != nil || ev.Amount == 10000 {
		t.Fatalf("amount=%d err=%v", ev.Amount, err)
	}
}

func TestRejectedScenarioRefusesAtInitiation(t *testing.T) {
	s, _ := newSim(t)
	if _, err := collect(t, s, "08"); !errors.Is(err, operators.ErrRejected) {
		t.Fatalf("got %v", err)
	}
}

func TestExpiringScenarioEndsExpiredAtTheOperator(t *testing.T) {
	s, clk := newSim(t)
	ref, _ := collect(t, s, "09")
	clk.advance(6 * time.Second)
	if statusOf(t, s, ref) != operators.StateExpired {
		t.Fatal("the operator must report the request expired")
	}
}
