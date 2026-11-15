package main

import (
	"os"
	"path/filepath"
	"software-studio/internal/configuration"
	"testing"
)

func TestLocalModuleBoundary(t *testing.T) {
	dir := t.TempDir()
	header := `language "1.0"; module "test.main" version "1.0.0";`
	entry := filepath.Join(dir, "main.lce")
	if e := os.WriteFile(entry, []byte(header), 0600); e != nil {
		t.Fatal(e)
	}
	p, e := loadModules(entry)
	if e != nil || !configuration.Compile(p).Valid {
		t.Fatal(e)
	}
	outside := filepath.Join(t.TempDir(), "outside.lce")
	_ = os.WriteFile(outside, []byte(header), 0600)
	if e = os.Symlink(outside, filepath.Join(dir, "linked.lce")); e != nil {
		t.Fatal(e)
	}
	_ = os.WriteFile(entry, []byte(header+` import "linked.lce" as linked;`), 0600)
	if _, e = loadModules(entry); e == nil {
		t.Fatal("symlink escape accepted")
	}
}
