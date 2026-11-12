// datacheck validates portable data contracts, complete fixtures and compatibility
// without a server, catalog lookup or database connection.
package main

import (
	"bytes"
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
	mode := "contract"
	if len(args) == 2 && (args[0] == "--lock" || args[0] == "--fixture" || args[0] == "--compare") {
		mode, args = args[0], args[1:]
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: datacheck [--lock|--fixture|--compare] <file.json|->")
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
		return fmt.Errorf("invalid contract at %s: %s", issues[0].Path, issues[0].Message)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	switch mode {
	case "contract":
		var contract model.DataContract
		if err := decode(&contract); err != nil {
			return err
		}
		if err := issuesError(model.ValidateDataContract(contract)); err != nil {
			return err
		}
		digest, err := model.DataContractDigest(contract)
		if err != nil {
			return err
		}
		return enc.Encode(model.DataContractLock{Contract: contract, Digest: digest})
	case "--lock":
		var lock model.DataContractLock
		if err := decode(&lock); err != nil {
			return err
		}
		if err := issuesError(model.ValidateDataContractLock(lock)); err != nil {
			return err
		}
		return enc.Encode(lock)
	case "--fixture":
		var example model.DataExample
		if err := decode(&example); err != nil {
			return err
		}
		result, err := model.CheckDataFixture(example.Contract, example.Fixture)
		if err != nil {
			return err
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
		if !result.Valid {
			return fmt.Errorf("fixture violates its data contract")
		}
		return nil
	case "--compare":
		var pair struct {
			Old model.DataContractLock `json:"old"`
			New model.DataContractLock `json:"new"`
		}
		if err := decode(&pair); err != nil {
			return err
		}
		result, err := model.CompareDataContracts(pair.Old, pair.New)
		if err != nil {
			return err
		}
		if err := enc.Encode(result); err != nil {
			return err
		}
		if !result.Compatible {
			return fmt.Errorf("new contract has potentially breaking changes")
		}
		return nil
	}
	return fmt.Errorf("unsupported mode")
}
