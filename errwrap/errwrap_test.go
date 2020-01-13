package errwrap_test

import (
	"testing"

	"github.com/fatih/errwrap/errwrap"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, errwrap.Analyzer, "a")
}
