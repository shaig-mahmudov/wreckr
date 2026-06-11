package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/report"
	"github.com/wreckr/wreckr/apps/api/internal/runner"
	"github.com/wreckr/wreckr/apps/api/internal/scenario"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	jsonOutput := fs.Bool("json", false, "print JSON report")
	timeout := fs.Duration("timeout", 5*time.Minute, "maximum run duration")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "scenario file is required")
		fmt.Fprintln(os.Stderr, "usage: wreckr run [flags] <scenario.json>")
		os.Exit(2)
	}

	sc, err := scenario.LoadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	rep, err := runner.New().Run(ctx, sc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *jsonOutput {
		raw, err := rep.JSON()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(string(raw))
	} else {
		fmt.Print(rep.Text())
	}

	if rep.Status != report.StatusPassed {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Wreckr - production scenario testing for backend systems")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  run <scenario.json>   run a scenario file")
}
