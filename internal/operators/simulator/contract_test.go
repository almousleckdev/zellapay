package simulator

import (
	"testing"

	"github.com/zellapay/zellapay/internal/operators"
	"github.com/zellapay/zellapay/internal/operators/operatorstest"
)

func TestSimulatorSatisfiesTheAdapterContract(t *testing.T) {
	operatorstest.Run(t, operatorstest.Suite{
		New: func(t *testing.T) operators.Adapter {
			s, _ := newSim(t)
			return s
		},
		Phone: phone("00"),
	})
}
