// sourcecheck compiles bounded LCE Assembly into the existing system JSON format.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"software-studio/internal/source"
)

func main() {
	var input io.Reader = os.Stdin
	if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "usage: sourcecheck [file.lce]")
		os.Exit(2)
	}
	if len(os.Args) == 2 {
		f, err := os.Open(os.Args[1])
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
	if err := encoder.Encode(r.Design); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
