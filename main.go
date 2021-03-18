package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/fatih/errwrap/errwrap"
	"golang.org/x/tools/go/analysis/singlechecker"
)

var (
	version string
	date    string
)

func main() {
	// this is a small hack to implement the -V flag that is part of
	// go/analysis framework. It'll allow us to print the version with -V, but
	// the --help message will print the flags of the analyzer
	ff := flag.NewFlagSet("errwrap", flag.ContinueOnError)
	v := ff.Bool("V", false, "print version and exit")
	ff.Usage = func() {}
	ff.SetOutput(io.Discard)

	ff.Parse(os.Args[1:])
	if *v {
		fmt.Printf("errwrap version %s (%s)\n", version, date)
		os.Exit(0)
	}
	singlechecker.Main(errwrap.Analyzer)
}
