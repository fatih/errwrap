package errwrap

import (
	internal "github.com/fatih/errwrap/internal/errwrap"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

// Analyzer of the linter
var Analyzer = &analysis.Analyzer{
	Name:             "errwrap",
	Doc:              "wrap errors in fmt.Errorf() calls with the %w verb directive",
	Requires:         []*analysis.Analyzer{inspect.Analyzer},
	Run:              internal.Run,
	RunDespiteErrors: true,
}
