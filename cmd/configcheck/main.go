// configcheck compiles, tests, packages and reproduces closed source projects.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"software-studio/internal/configuration"
	"strings"
	"time"
)

func main() {
	mode := flag.String("mode", "test", "compile, test, pack, verify, replay, example or migrate")
	file := flag.String("file", "", "project/package JSON or legacy .lce file (stdin when omitted)")
	out := flag.String("out", "", "output JSON path (stdout when omitted)")
	id := flag.String("id", "local.configuration", "package ID")
	version := flag.String("version", "1.0.0", "package version")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var result any
	var err error
	var b []byte
	if *mode != "example" {
		var reader io.Reader = os.Stdin
		if *file != "" {
			f, e := os.Open(*file)
			if e != nil {
				fatal(e)
			}
			defer f.Close()
			reader = f
		}
		b, err = io.ReadAll(io.LimitReader(reader, (32<<20)+1))
		if err != nil {
			fatal(err)
		}
		if len(b) > 32<<20 {
			fatal(fmt.Errorf("input exceeds 32 MiB"))
		}
	}
	failed := false
	switch *mode {
	case "example":
		result, err = configuration.InspectionProject()
	case "migrate":
		result, err = configuration.MigrateLegacy(string(b))
	case "verify", "replay":
		var p configuration.Package
		if err = json.Unmarshal(b, &p); err != nil {
			break
		}
		if *mode == "verify" {
			result, err = configuration.VerifyPackage(p)
		} else {
			var r configuration.Report
			r, err = configuration.Replay(ctx, p)
			result = r
			failed = !r.Passed
		}
	case "compile", "test", "pack":
		var p configuration.Project
		if strings.HasSuffix(*file, ".lce") {
			p, err = loadModules(*file)
		} else {
			err = json.Unmarshal(b, &p)
		}
		if err != nil {
			break
		}
		switch *mode {
		case "compile":
			c := configuration.Compile(p)
			result = c
			failed = !c.Valid
		case "test":
			var r configuration.Report
			r, err = configuration.Run(ctx, p)
			result = r
			failed = !r.Passed
		case "pack":
			var pkg configuration.Package
			pkg, err = configuration.Pack(ctx, *id, *version, p, true)
			result = pkg
			failed = pkg.Report == nil || !pkg.Report.Passed
		}
	default:
		err = fmt.Errorf("unknown mode %s", *mode)
	}
	if err != nil {
		fatal(err)
	}
	var writer io.Writer = os.Stdout
	if *out != "" {
		f, e := os.Create(*out)
		if e != nil {
			fatal(e)
		}
		defer f.Close()
		writer = f
	}
	enc := json.NewEncoder(writer)
	enc.SetIndent("", "  ")
	if e := enc.Encode(result); e != nil {
		fatal(e)
	}
	if failed {
		os.Exit(1)
	}
}
func fatal(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(2) }
