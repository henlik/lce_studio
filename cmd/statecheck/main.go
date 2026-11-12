// statecheck validates portable state machines and runs bounded state scenarios
// without a server, catalog lookup, database connection or external effects.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"software-studio/internal/model"
	"software-studio/internal/state"
)

func main() {
	if err := check(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(args []string, stdin io.Reader, stdout io.Writer) error {
	mode := "machine"
	if len(args) == 2 && (args[0] == "--lock" || args[0] == "--run") {
		mode, args = args[0], args[1:]
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: statecheck [--lock|--run] <file.json|->")
	}
	input := stdin
	if args[0] != "-" {
		file, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("open input: %w", err)
		}
		defer file.Close()
		input = file
	}
	const maxBytes = 2 << 20
	data, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if len(data) > maxBytes {
		return fmt.Errorf("input exceeds 2 MiB")
	}
	decode := func(value any) error {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(value); err != nil {
			return fmt.Errorf("decode input: %w", err)
		}
		if err := dec.Decode(new(any)); err != io.EOF {
			return fmt.Errorf("input must contain exactly one JSON value")
		}
		return nil
	}
	issuesError := func(issues []model.ValidationIssue) error {
		if len(issues) == 0 {
			return nil
		}
		return fmt.Errorf("invalid machine at %s: %s", issues[0].Path, issues[0].Message)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	switch mode {
	case "machine":
		var machine model.StateMachine
		if err := decode(&machine); err != nil {
			return err
		}
		if err := issuesError(model.ValidateStateMachine(machine)); err != nil {
			return err
		}
		digest, err := model.StateMachineDigest(machine)
		if err != nil {
			return err
		}
		return enc.Encode(model.StateMachineLock{Machine: machine, Digest: digest})
	case "--lock":
		var lock model.StateMachineLock
		if err := decode(&lock); err != nil {
			return err
		}
		if err := issuesError(model.ValidateStateMachineLock(lock)); err != nil {
			return err
		}
		return enc.Encode(lock)
	case "--run":
		var example model.StateExample
		if err := decode(&example); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result, err := state.Run(ctx, example.Machine, example.Scenario)
		if err != nil {
			return err
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
		if result.Verified && !result.Passed {
			return fmt.Errorf("scenario failed its declared final-state expectations")
		}
		return nil
	}
	return fmt.Errorf("unsupported mode")
}
