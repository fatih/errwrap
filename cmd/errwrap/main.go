package main

import (
	"github.com/fatih/errwrap/internal/errwrap"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(errwrap.Analyzer)
}
