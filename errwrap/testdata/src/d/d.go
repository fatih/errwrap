package d

import (
	"errors"
	"fmt"
)

func foo() error {
	err := errors.New("bar!")
	err2 := errors.New("bar!")
	return fmt.Errorf("failed with errors %w, %w", err, err2) // want `Errorf call has more than one error-wrapping directive %w`
}

