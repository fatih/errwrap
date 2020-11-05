package e

import (
	"errors"
	"fmt"
)

func foo() error {
	err := errors.New("bar!")
	return fmt.Errorf("failed for %s with error: %s", "foo", err.Error()) // want `call could wrap the error with error-wrapping directive %w`
}
