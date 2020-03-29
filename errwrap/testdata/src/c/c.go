package c

import (
	"errors"
	"fmt"
)

func foo() error {
	err := errors.New("bar!")
	return fmt.Errorf("failed for %s with error: ", "foo", err) // want `Errorf call needs 1 arg but has 2 args`
}
