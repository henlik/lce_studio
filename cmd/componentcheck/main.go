// componentcheck validates a declarative component and prints its portable lock.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"software-studio/internal/model"
)

func main() {
	if err := check(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: componentcheck <package.json|->")
	}
	input := stdin
	if args[0] != "-" {
		file, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open package: %w", err)
		}
		defer file.Close()
		input = file
	}
	const maxBytes = 2 << 20
	bytes, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return fmt.Errorf("read package: %w", err)
	}
	if len(bytes) > maxBytes {
		return fmt.Errorf("package exceeds the 2 MiB input limit")
	}
	var pkg model.ComponentPackage
	// The CLI applies the same unknown-field and single-value rules as the API.
	if err := decodePackage(bytes, &pkg); err != nil {
		return err
	}
	if issues := model.ValidateComponentPackage(pkg); len(issues) > 0 {
		return fmt.Errorf("invalid package: %s: %s", issues[0].Path, issues[0].Message)
	}
	digest, err := model.ComponentDigest(pkg)
	if err != nil {
		return fmt.Errorf("compute package digest: %w", err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(model.PackageLock{Package: pkg, Digest: digest})
}
