# errwrap

Wrap and fix Go errors with the new %w verb directive. This tool analyzes
`fmt.Errorf()` calls and reports calls that contain a verb directive that is
different than the new `%w` verb directive [introduced in Go v1.13](https://golang.org/doc/go1.13#error_wrapping). It's also capable of rewriting calls to use the new `%w` wrap verb directive.

![errwrap](https://user-images.githubusercontent.com/438920/69905498-26b34c80-1369-11ea-888d-608f32678971.gif)

# Install

```bash
go get github.com/fatih/errwrap/cmd/errwrap
```

# Usage

By default, `errwrap` prints the output of the analyzer to stdout. You can pass
a file, directory or a Go package:

```sh
$ errwrap foo.go # pass a file
$ errwrap ./...  # recursively analyze all files
$ errwrap github.com/fatih/gomodifytags # or pass a package
```

When called it displays the error with the line and column:

```
gomodifytags@v1.0.1/main.go:200:16: call could wrap the error with error-wrapping directive %w
gomodifytags@v1.0.1/main.go:641:17: call could wrap the error with error-wrapping directive %w
gomodifytags@v1.0.1/main.go:749:15: call could wrap the error with error-wrapping directive %w
```

`errwrap` is also able to rewrite your source code to replace any verb
directive used for an `error` type with the `%w` verb directive. Assume we have
the following source code:

```
$ cat demo.go
package main

import (
        "errors"
        "fmt"
)

func main() {
        _ = foo()
}

func foo() error {
        err := errors.New("bar!")
        return fmt.Errorf("foo failed: %s: %w bar ...", "foo", err)
}
```

Calling `errwrap` with the `-fix` flag will rewrite the source code:

```
$ errwrap -fix example.go
demo.go:14:9: call could wrap the error with error-wrapping directive %w
```
