package simulator

import "time"

// scenario is the scripted behaviour selected by a phone number. It lets
// developers and the sandbox trigger every operator outcome on demand.
type scenario int8

const (
	scenSucceed           scenario = iota // ends in 00, also the default
	scenDeclined                          // 01: customer declines
	scenInsufficientFunds                 // 02: customer lacks funds
	scenTimeoutThenOK                     // 03: initiation times out, then succeeds
	scenNeverCompletes                    // 04: stays pending forever
	scenDuplicateCallback                 // 05: success, callback sent twice
	scenMissingCallback                   // 06: success, callback never sent
	scenWrongAmount                       // 07: success, callback amount differs
	scenRejected                          // 08: operator rejects the request outright
	scenExpires                           // 09: customer never answers; operator expires it
)

var bySuffix = map[string]scenario{
	"01": scenDeclined,
	"02": scenInsufficientFunds,
	"03": scenTimeoutThenOK,
	"04": scenNeverCompletes,
	"05": scenDuplicateCallback,
	"06": scenMissingCallback,
	"07": scenWrongAmount,
	"08": scenRejected,
	"09": scenExpires,
}

// scenarioFor reads the last two digits of an E.164 phone number.
func scenarioFor(phone string) scenario {
	if len(phone) < 2 {
		return scenSucceed
	}
	if s, ok := bySuffix[phone[len(phone)-2:]]; ok {
		return s
	}
	return scenSucceed
}

// callbackCount is how many times the operator calls back once final.
func (s scenario) callbackCount() int {
	switch s {
	case scenNeverCompletes, scenMissingCallback:
		return 0
	case scenDuplicateCallback:
		return 2
	}
	return 1
}

// Config tunes the simulator.
type Config struct {
	// Delay is how long a transaction stays pending before it resolves.
	Delay time.Duration
	// Secret signs callbacks, as a real operator's callback secret would.
	Secret []byte
}
