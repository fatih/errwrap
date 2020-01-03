// Package errwrap defines an Analyzer that rewrites error statements to use
// the new wrapping/unwrapping functionality
package errwrap

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/printer"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Analyzer of the linter
var Analyzer = &analysis.Analyzer{
	Name:             "errwrap",
	Doc:              "wrap errors in fmt.Errorf() calls with the %w verb directive",
	Requires:         []*analysis.Analyzer{inspect.Analyzer},
	Run:              run,
	RunDespiteErrors: true,
}

// Run is the runner for an analysis pass
func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		call := n.(*ast.CallExpr)

		fn, _ := typeutil.Callee(pass.TypesInfo, call).(*types.Func)
		if fn == nil {
			return
		}

		// for now only check these functions
		if fn.FullName() != "fmt.Errorf" {
			return
		}

		oldExpr := render(pass.Fset, call)

		format, idx := formatString(pass, call)
		if idx < 0 {
			// call has arguments but no formatting directives
			return
		}

		firstArg := idx + 1 // Arguments are immediately after format string.
		if !strings.Contains(format, "%") {
			if len(call.Args) > firstArg {
				pass.Reportf(call.Lparen, "%s call has arguments but no formatting directives", fn.Name())
			}
			return
		}

		var hasError bool
		var errIndex int
		for i, arg := range call.Args {
			if t := pass.TypesInfo.TypeOf(arg); t != nil {
				if t.String() == "error" {
					hasError = true
					errIndex = i
				}
			}
		}

		if !hasError {
			return
		}

		argNum := firstArg
		maxArgNum := firstArg
		anyIndex := false
		anyW := false
		newFormat := []byte(format)
		for i, w := 0, 0; i < len(format); i += w {
			w = 1
			if format[i] != '%' {
				continue
			}

			state := parsePrintfVerb(pass, call, fn.Name(), format[i:], firstArg, argNum)
			if state == nil {
				return
			}

			w = len(state.format)
			if state.hasIndex {
				anyIndex = true
			}

			if len(state.argNums) > 0 {
				// Continue with the next sequential argument.
				argNum = state.argNums[len(state.argNums)-1] + 1
			}

			for _, n := range state.argNums {
				if n >= maxArgNum {
					maxArgNum = n + 1
				}
			}

			if state.argNum != errIndex {
				continue
			}

			if state.verb == 'w' {
				if anyW {
					pass.Reportf(call.Pos(), "%s call has more than one error-wrapping directive %%w", state.name)
					return
				}
				anyW = true
				continue
			}

			newFormat[i+1] = 'w'

			if bl, ok := call.Args[0].(*ast.BasicLit); ok {
				// replace the expression, keep the arguments the same
				call.Args[0] = &ast.BasicLit{
					Value:    strconv.Quote(string(newFormat)),
					ValuePos: bl.ValuePos,
					Kind:     bl.Kind,
				}
			}

			newExpr := render(pass.Fset, call)

			pass.Report(analysis.Diagnostic{
				Pos:     call.Pos(),
				Message: "call could wrap the error with error-wrapping directive %w",
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: fmt.Sprintf("should replace `%s` with `%s`", oldExpr, newExpr),
						TextEdits: []analysis.TextEdit{
							{
								Pos:     call.Pos(),
								End:     call.End(),
								NewText: []byte(newExpr),
							},
						},
					},
				},
			})
		}

		// Dotdotdot is hard.
		if call.Ellipsis.IsValid() && maxArgNum >= len(call.Args)-1 {
			return
		}
		// If any formats are indexed, extra arguments are ignored.
		if anyIndex {
			return
		}
		// There should be no leftover arguments.
		if maxArgNum != len(call.Args) {
			expect := maxArgNum - firstArg
			numArgs := len(call.Args) - firstArg
			pass.Reportf(call.Pos(), "%s call needs %v but has %v", fn.Name(), count(expect, "arg"), count(numArgs, "arg"))
		}

		// If any formats are indexed, extra arguments are ignored.
		if anyIndex {
			return
		}

		return
	})

	return nil, nil
}

// render returns the pretty-print of the given node
func render(fset *token.FileSet, x interface{}) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		panic(err)
	}
	return buf.String()
}

//
// NOTE(arslan): Copied from go/analysis/passes/printf/printf.go
//

// formatState holds the parsed representation of a printf directive such as "%3.*[4]d".
// It is constructed by parsePrintfVerb.
type formatState struct {
	verb     rune   // the format verb: 'd' for "%d"
	format   string // the full format directive from % through verb, "%.3d".
	name     string // Printf, Sprintf etc.
	flags    []byte // the list of # + etc.
	argNums  []int  // the successive argument numbers that are consumed, adjusted to refer to actual arg in call
	firstArg int    // Index of first argument after the format in the Printf call.
	pos      int    // index of the verb in the format string

	// Used only during parse.
	pass         *analysis.Pass
	call         *ast.CallExpr
	argNum       int  // Which argument we're expecting to format now.
	hasIndex     bool // Whether the argument is indexed.
	indexPending bool // Whether we have an indexed argument that has not resolved.
	nbytes       int  // number of bytes of the format string consumed.
}

// formatString returns the format string argument and its index within
// the given printf-like call expression.
//
// The last parameter before variadic arguments is assumed to be
// a format string.
//
// The first string literal or string constant is assumed to be a format string
// if the call's signature cannot be determined.
//
// If it cannot find any format string parameter, it returns ("", -1).
func formatString(pass *analysis.Pass, call *ast.CallExpr) (format string, idx int) {
	typ := pass.TypesInfo.Types[call.Fun].Type
	if typ != nil {
		if sig, ok := typ.(*types.Signature); ok {
			if !sig.Variadic() {
				// Skip checking non-variadic functions.
				return "", -1
			}
			idx := sig.Params().Len() - 2
			if idx < 0 {
				// Skip checking variadic functions without
				// fixed arguments.
				return "", -1
			}
			s, ok := stringConstantArg(pass, call, idx)
			if !ok {
				// The last argument before variadic args isn't a string.
				return "", -1
			}
			return s, idx
		}
	}

	// Cannot determine call's signature. Fall back to scanning for the first
	// string constant in the call.
	for idx := range call.Args {
		if s, ok := stringConstantArg(pass, call, idx); ok {
			return s, idx
		}
		if pass.TypesInfo.Types[call.Args[idx]].Type == types.Typ[types.String] {
			// Skip checking a call with a non-constant format
			// string argument, since its contents are unavailable
			// for validation.
			return "", -1
		}
	}
	return "", -1
}

// stringConstantArg returns call's string constant argument at the index idx.
//
// ("", false) is returned if call's argument at the index idx isn't a string
// constant.
func stringConstantArg(pass *analysis.Pass, call *ast.CallExpr, idx int) (string, bool) {
	if idx >= len(call.Args) {
		return "", false
	}
	arg := call.Args[idx]
	lit := pass.TypesInfo.Types[arg].Value
	if lit != nil && lit.Kind() == constant.String {
		return constant.StringVal(lit), true
	}
	return "", false
}

// parsePrintfVerb looks the formatting directive that begins the format string
// and returns a formatState that encodes what the directive wants, without looking
// at the actual arguments present in the call. The result is nil if there is an error.
func parsePrintfVerb(pass *analysis.Pass, call *ast.CallExpr, name, format string, firstArg, argNum int) *formatState {
	state := &formatState{
		format:   format,
		name:     name,
		flags:    make([]byte, 0, 5),
		argNum:   argNum,
		argNums:  make([]int, 0, 1),
		nbytes:   1, // There's guaranteed to be a percent sign.
		firstArg: firstArg,
		pass:     pass,
		call:     call,
	}
	// There may be flags.
	state.parseFlags()
	// There may be an index.
	if !state.parseIndex() {
		return nil
	}
	// There may be a width.
	if !state.parseNum() {
		return nil
	}
	// There may be a precision.
	if !state.parsePrecision() {
		return nil
	}
	// Now a verb, possibly prefixed by an index (which we may already have).
	if !state.indexPending && !state.parseIndex() {
		return nil
	}

	if state.nbytes == len(state.format) {
		pass.Reportf(call.Pos(), "%s format %s is missing verb at end of string", name, state.format)
		return nil
	}
	verb, w := utf8.DecodeRuneInString(state.format[state.nbytes:])
	state.verb = verb
	state.nbytes += w
	if verb != '%' {
		state.argNums = append(state.argNums, state.argNum)
	}
	state.format = state.format[:state.nbytes]
	return state
}

// parseFlags accepts any printf flags.
func (s *formatState) parseFlags() {
	for s.nbytes < len(s.format) {
		switch c := s.format[s.nbytes]; c {
		case '#', '0', '+', '-', ' ':
			s.flags = append(s.flags, c)
			s.nbytes++
		default:
			return
		}
	}
}

// scanNum advances through a decimal number if present.
func (s *formatState) scanNum() {
	for ; s.nbytes < len(s.format); s.nbytes++ {
		c := s.format[s.nbytes]
		if c < '0' || '9' < c {
			return
		}
	}
}

// parseIndex scans an index expression. It returns false if there is a syntax error.
func (s *formatState) parseIndex() bool {
	if s.nbytes == len(s.format) || s.format[s.nbytes] != '[' {
		return true
	}
	// Argument index present.
	s.nbytes++ // skip '['
	start := s.nbytes
	s.scanNum()
	ok := true
	if s.nbytes == len(s.format) || s.nbytes == start || s.format[s.nbytes] != ']' {
		ok = false
		s.nbytes = strings.Index(s.format, "]")
		if s.nbytes < 0 {
			s.pass.Reportf(s.call.Pos(), "%s format %s is missing closing ]", s.name, s.format)
			return false
		}
	}
	arg32, err := strconv.ParseInt(s.format[start:s.nbytes], 10, 32)
	if err != nil || !ok || arg32 <= 0 || arg32 > int64(len(s.call.Args)-s.firstArg) {
		s.pass.Reportf(s.call.Pos(), "%s format has invalid argument index [%s]", s.name, s.format[start:s.nbytes])
		return false
	}
	s.nbytes++ // skip ']'
	arg := int(arg32)
	arg += s.firstArg - 1 // We want to zero-index the actual arguments.
	s.argNum = arg
	s.hasIndex = true
	s.indexPending = true
	return true
}

// parseNum scans a width or precision (or *). It returns false if there's a bad index expression.
func (s *formatState) parseNum() bool {
	if s.nbytes < len(s.format) && s.format[s.nbytes] == '*' {
		if s.indexPending { // Absorb it.
			s.indexPending = false
		}
		s.nbytes++
		s.argNums = append(s.argNums, s.argNum)
		s.argNum++
	} else {
		s.scanNum()
	}
	return true
}

// parsePrecision scans for a precision. It returns false if there's a bad index expression.
func (s *formatState) parsePrecision() bool {
	// If there's a period, there may be a precision.
	if s.nbytes < len(s.format) && s.format[s.nbytes] == '.' {
		s.flags = append(s.flags, '.') // Treat precision as a flag.
		s.nbytes++
		if !s.parseIndex() {
			return false
		}
		if !s.parseNum() {
			return false
		}
	}
	return true
}

// count(n, what) returns "1 what" or "N whats"
// (assuming the plural of what is whats).
func count(n int, what string) string {
	if n == 1 {
		return "1 " + what
	}
	return fmt.Sprintf("%d %ss", n, what)
}
