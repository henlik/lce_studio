// Command conformance runs the bounded reception experiment against PostgreSQL.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"software-studio/internal/conformance"
	"software-studio/internal/system"
)

func main() {
	path := flag.String("design", "", "Optional system design JSON; defaults to reception reference")
	flag.Parse()
	d := system.ReceptionExample()
	if *path != "" {
		b, err := os.ReadFile(*path)
		if err != nil {
			fail(err)
		}
		d = system.SystemDesign{}
		decoder := json.NewDecoder(bytes.NewReader(b))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&d); err != nil {
			fail(err)
		}
		var trailing any
		if err = decoder.Decode(&trailing); err != io.EOF {
			fail(fmt.Errorf("design must contain exactly one JSON object"))
		}
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		fail(fmt.Errorf("DATABASE_URL is required"))
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		fail(fmt.Errorf("invalid PostgreSQL configuration"))
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	report, err := conformance.Run(ctx, db, d)
	if err != nil {
		fail(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(report); err != nil {
		fail(err)
	}
	if !report.Passed {
		os.Exit(1)
	}
}
func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
