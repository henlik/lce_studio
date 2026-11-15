// sourcecheck compiles bounded LCE source and executes offline testbenches.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"software-studio/internal/source"
	"time"
)

func main() {
	run := flag.Bool("test", false, "run the component testbench; emit source, model and results as JSON")
	flag.Parse()
	var input io.Reader = os.Stdin
	if flag.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "usage: sourcecheck [-test] [file.lce]")
		os.Exit(2)
	}
	if flag.NArg() == 1 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer f.Close()
		input = f
	}
	b, err := io.ReadAll(io.LimitReader(input, source.MaxBytes+1))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	r := source.Compile(string(b))
	if !r.Valid {
		for _, d := range r.Diagnostics {
			fmt.Fprintf(os.Stderr, "%d:%d %s: %s\n", d.Line, d.Column, d.Path, d.Message)
		}
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	var output any = r.Design
	passed := true
	if r.Component != nil {
		output = r
	}
	if *run {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		report, err := source.RunTestbench(ctx, string(b))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		output = report
		passed = report.Passed
	}
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if !passed {
		os.Exit(1)
	}
}
