package b

import (
	"errors"
	"fmt"
)

func foo() error {
	err := errors.New("bar!")
	return fmt.Errorf("failed for with error: ", err) // want `Errorf call has arguments but no formatting directives`
}
