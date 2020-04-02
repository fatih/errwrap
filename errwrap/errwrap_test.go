package errwrap_test

import (
	"testing"

	"github.com/fatih/errwrap/errwrap"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()

	for _, tcase := range []struct {
		name string
		dir  string
	}{
		{
			name: "wrap the error with error-wrapping directive",
			dir:  "a",
		},
		{
			name: "has arguments but no formatting directives",
			dir:  "b",
		},
		{
			name: "has leftover arguments",
			dir:  "c",
		},
		{
			name: "too many formatting directives",
			dir:  "d",
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			analysistest.Run(t, testdata, errwrap.Analyzer, tcase.dir)
		})
	}
}
