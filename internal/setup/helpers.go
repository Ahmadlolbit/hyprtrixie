package setup

import (
	"github.com/Ahmadlolbit/hyprtrixie/internal/run"
)

func holds(ok bool) error {
	if !ok {
		return run.Reported("")
	}
	return nil
}
