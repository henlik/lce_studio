// componentcheck validates profile locks or composite bundles and runs root tests.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"software-studio/internal/behavior"
	"software-studio/internal/model"
)

func main() {
	if err := check(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(args []string, stdin io.Reader, stdout io.Writer) error {
	mode := "profile"
	if len(args) == 2 && (args[0] == "--bundle" || args[0] == "--test") {
		mode, args = args[0], args[1:]
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: componentcheck [--bundle|--test] <file.json|->")
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
	data, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return fmt.Errorf("read package: %w", err)
	}
	if len(data) > maxBytes {
		return fmt.Errorf("package exceeds the 2 MiB input limit")
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if mode != "profile" {
		var bundle model.ComponentBundle
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&bundle); err != nil {
			return fmt.Errorf("decode bundle: %w", err)
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return fmt.Errorf("bundle must contain exactly one JSON value")
		}
		if issues := model.ValidateComponentBundle(bundle); len(issues) > 0 {
			return fmt.Errorf("invalid bundle: %s: %s", issues[0].Path, issues[0].Message)
		}
		if mode == "--bundle" {
			return encoder.Encode(struct {
				Valid    bool               `json:"valid"`
				Root     model.ComponentRef `json:"root"`
				Packages int                `json:"packages"`
			}{true, bundle.Root, len(bundle.Packages)})
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result, err := behavior.Test(ctx, bundle)
		if err != nil {
			return fmt.Errorf("test bundle: %w", err)
		}
		if err := encoder.Encode(result); err != nil {
			return err
		}
		if !result.Passed {
			return fmt.Errorf("component tests did not pass (an empty test suite is unverified)")
		}
		return nil
	}
	var pkg model.ComponentPackage
	// The CLI applies the same unknown-field and single-value rules as the API.
	if err := decodePackage(data, &pkg); err != nil {
		return err
	}
	if issues := model.ValidateComponentPackage(pkg); len(issues) > 0 {
		return fmt.Errorf("invalid package: %s: %s", issues[0].Path, issues[0].Message)
	}
	if pkg.SchemaVersion != 1 {
		return fmt.Errorf("composite packages require their exact dependency bundle; use --bundle or --test")
	}
	digest, err := model.ComponentDigest(pkg)
	if err != nil {
		return fmt.Errorf("compute package digest: %w", err)
	}
	return encoder.Encode(model.PackageLock{Package: pkg, Digest: digest})
}
