// Command doctor scans a Go project's own source for vendor SDKs it already
// imports (an HTTP router, a database driver, a message broker client, ...)
// and reports any of them that don't yet have a matching Argos integration
// imported alongside them.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	strict := flag.Bool("strict", false, "exit with status 1 if any integration is suggested (for CI)")
	initFlag := flag.Bool("init", false, "generate a starter argos.config.yaml and print a main.go wiring snippet, instead of scanning for suggestions")
	force := flag.Bool("force", false, "with -init, overwrite an existing argos.config.yaml")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	if *initFlag {
		if err := runInit(dir, *force, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "argos-doctor:", err)
			os.Exit(2)
		}
		return
	}

	n, err := run(dir, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "argos-doctor:", err)
		os.Exit(2)
	}
	if *strict && n > 0 {
		os.Exit(1)
	}
}

func run(dir string, out io.Writer) (int, error) {
	imports, err := CollectImports(dir)
	if err != nil {
		return 0, err
	}

	suggestions := Suggest(imports)

	if _, err := fmt.Fprintf(out, "Argos Doctor - scanned %s\n\n", dir); err != nil {
		return 0, err
	}
	if len(suggestions) == 0 {
		_, err := fmt.Fprintln(out, "Nothing to suggest - every vendor SDK detected already has a matching Argos integration imported.")
		return 0, err
	}
	for _, s := range suggestions {
		if _, err := fmt.Fprintln(out, s.String()); err != nil {
			return 0, err
		}
	}
	if _, err := fmt.Fprintf(out, "\n%d integration(s) suggested.\n", len(suggestions)); err != nil {
		return 0, err
	}
	return len(suggestions), nil
}
