package main

import (
	"fmt"

	"golang.org/x/tools/go/packages"
)

// CollectImports loads every package under dir ("./..." from dir) and
// returns the set of import paths that appear directly in the project's own
// source files, including test files. It deliberately does not follow
// imports transitively past that first level: whether some dependency of a
// dependency happens to import, say, Kafka's client library internally is
// not evidence the project itself uses Kafka, and would produce false
// suggestions.
func CollectImports(dir string) (map[string]bool, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedImports,
		Dir:   dir,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages in %s: %w", dir, err)
	}

	imports := make(map[string]bool)
	for _, p := range pkgs {
		imports[p.PkgPath] = true
		for path := range p.Imports {
			imports[path] = true
		}
	}
	return imports, nil
}
