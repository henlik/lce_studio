// workflowcheck validates portable workflow packages and runs bounded workflow
// experiments without a server, catalog lookup or database connection.
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
	"software-studio/internal/workflow"
)

func main() {
	if err := check(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(args []string, stdin io.Reader, stdout io.Writer) error {
	mode := "definition"
	if len(args) == 2 && (args[0] == "--lock" || args[0] == "--run") {
		mode, args = args[0], args[1:]
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: workflowcheck [--lock|--run] <file.json|->")
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
		return fmt.Errorf("invalid workflow package at %s: %s", issues[0].Path, issues[0].Message)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	switch mode {
	case "definition":
		var definition model.WorkflowDefinition
		if err := decode(&definition); err != nil {
			return err
		}
		if err := issuesError(model.ValidateWorkflowDefinition(definition)); err != nil {
			return err
		}
		digest, err := model.WorkflowDigest(definition)
		if err != nil {
			return err
		}
		return enc.Encode(model.WorkflowLock{Workflow: definition, Digest: digest})
	case "--lock":
		var lock model.WorkflowLock
		if err := decode(&lock); err != nil {
			return err
		}
		if err := issuesError(model.ValidateWorkflowLock(lock)); err != nil {
			return err
		}
		return enc.Encode(lock)
	case "--run":
		var example model.WorkflowExample
		if err := decode(&example); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result, err := workflow.Run(ctx, example.Workflow, example.Scenario)
		if err != nil {
			return err
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
		if result.Verified && !result.Passed {
			return fmt.Errorf("workflow scenario failed its independent expectations")
		}
		return nil
	}
	return fmt.Errorf("unsupported mode")
}
