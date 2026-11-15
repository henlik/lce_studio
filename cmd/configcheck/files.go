package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"software-studio/internal/configuration"
)

// os.Root confines local imports to the entry's directory, including symlinks.
// Only the CLI reads files; the compiler receives the same closed map as the API.
func loadModules(entry string) (configuration.Project, error) {
	root, e := os.OpenRoot(filepath.Dir(entry))
	if e != nil {
		return configuration.Project{}, e
	}
	defer root.Close()
	p := configuration.Project{Language: configuration.Version, Entry: filepath.Base(entry), Files: map[string]string{}}
	total := 0
	var load func(string) error
	load = func(file string) error {
		if _, ok := p.Files[file]; ok {
			return nil
		}
		if len(p.Files) >= configuration.MaxFiles {
			return fmt.Errorf("module file limit exceeded")
		}
		f, e := root.Open(file)
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(f, configuration.MaxFileBytes+1))
		f.Close()
		if e != nil {
			return e
		}
		total += len(b)
		if len(b) > configuration.MaxFileBytes || total > configuration.MaxProjectBytes {
			return fmt.Errorf("source byte limit exceeded")
		}
		p.Files[file] = string(b)
		d, ds := configuration.Parse(file, string(b))
		if len(ds) > 0 {
			return nil
		}
		for _, imp := range d.Imports {
			if e = load(path.Join(path.Dir(file), imp.Path)); e != nil {
				return e
			}
		}
		return nil
	}
	if e = load(p.Entry); e != nil {
		return p, e
	}
	if b, err := root.ReadFile("lock.json"); err == nil {
		if len(b) > 1<<20 {
			return p, fmt.Errorf("lockfile exceeds 1 MiB")
		}
		var lock configuration.Lockfile
		if err = json.Unmarshal(b, &lock); err != nil {
			return p, err
		}
		p.Lock = &lock
	} else if !errors.Is(err, os.ErrNotExist) {
		return p, err
	}
	return p, nil
}
