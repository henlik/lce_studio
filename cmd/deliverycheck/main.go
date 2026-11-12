// deliverycheck validates portable delivery models and runs bounded message
// experiments without a server, broker, catalog lookup or database connection.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"software-studio/internal/delivery"
	"software-studio/internal/model"
)

func main() {
	if err := check(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func check(args []string, stdin io.Reader, stdout io.Writer) error {
	mode := "model"
	if len(args) == 2 && (args[0] == "--lock" || args[0] == "--run") {
		mode, args = args[0], args[1:]
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: deliverycheck [--lock|--run] <file.json|->")
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
		return fmt.Errorf("invalid delivery model at %s: %s", issues[0].Path, issues[0].Message)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	switch mode {
	case "model":
		var definition model.DeliveryModel
		if err := decode(&definition); err != nil {
			return err
		}
		if err := issuesError(model.ValidateDeliveryModel(definition)); err != nil {
			return err
		}
		digest, err := model.DeliveryModelDigest(definition)
		if err != nil {
			return err
		}
		return enc.Encode(model.DeliveryModelLock{Model: definition, Digest: digest})
	case "--lock":
		var lock model.DeliveryModelLock
		if err := decode(&lock); err != nil {
			return err
		}
		if err := issuesError(model.ValidateDeliveryModelLock(lock)); err != nil {
			return err
		}
		return enc.Encode(lock)
	case "--run":
		var example model.DeliveryExample
		if err := decode(&example); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		result, err := delivery.Run(ctx, example.Model, example.Scenario)
		if err != nil {
			return err
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
		if result.Verified && !result.Passed {
			return fmt.Errorf("delivery scenario failed its independent expectations")
		}
		return nil
	}
	return fmt.Errorf("unsupported mode")
}
