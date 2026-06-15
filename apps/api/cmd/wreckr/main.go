package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/wreckr/wreckr/apps/api/internal/k6script"
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
	case "compile-k6":
		compileK6(os.Args[2:])
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

func compileK6(args []string) {
	fs := flag.NewFlagSet("compile-k6", flag.ExitOnError)
	output := fs.String("o", "", "write generated k6 script to this file")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "scenario file is required")
		fmt.Fprintln(os.Stderr, "usage: wreckr compile-k6 [-o script.js] <scenario.json>")
		os.Exit(2)
	}

	sc, err := scenario.LoadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	script, err := k6script.Compile(sc)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *output == "" {
		fmt.Print(script.Content)
		return
	}
	if err := os.WriteFile(*output, []byte(script.Content), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Wreckr - production scenario testing for backend systems")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "commands:")
	fmt.Fprintln(os.Stderr, "  run <scenario.json>              run a scenario file")
	fmt.Fprintln(os.Stderr, "  compile-k6 <scenario.json>       compile a scenario to a k6 script")
}
