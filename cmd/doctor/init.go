package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const configFileName = "argos.config.yaml"

// runInit generates a starter argos.config.yaml for the project at dir
// (refusing to overwrite an existing one unless force is set), then prints
// a ready-to-paste main.go wiring snippet plus the same integration
// suggestions the normal scan-and-report mode produces - reusing
// CollectImports/Suggest so the generated file and the printed advice never
// disagree with each other.
func runInit(dir string, force bool, out io.Writer) error {
	imports, err := CollectImports(dir)
	if err != nil {
		return err
	}
	suggestions := Suggest(imports)

	configPath := filepath.Join(dir, configFileName)
	if !force {
		if _, statErr := os.Stat(configPath); statErr == nil {
			return fmt.Errorf("%s already exists - pass -force to overwrite", configPath)
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
	}

	// database/sql is the only vendor system whose matching Argos module
	// (integrations/sql) currently has a WithYAMLConfig section - see
	// integrations/contrib/README.md for the pattern other modules can add.
	includeSQLSection := imports["database/sql"]

	if err := os.WriteFile(configPath, []byte(configYAML(moduleName(dir), includeSQLSection)), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", configPath, err)
	}

	if _, err := fmt.Fprintf(out, "Wrote %s\n\n%s\n", configPath, mainSnippet()); err != nil {
		return err
	}
	if len(suggestions) == 0 {
		_, err := fmt.Fprintln(out, "No additional Argos integrations suggested for this project's current imports.")
		return err
	}
	if _, err := fmt.Fprintln(out, "Also wire in:"); err != nil {
		return err
	}
	for _, s := range suggestions {
		if _, err := fmt.Fprintln(out, s.String()); err != nil {
			return err
		}
	}
	return nil
}

// moduleName reads the "module" line of dir/go.mod and returns its last
// path segment, falling back to dir's own base name if go.mod is missing,
// unreadable, or has no module line.
func moduleName(dir string) string {
	if data, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if after, ok := strings.CutPrefix(line, "module "); ok {
				after = strings.TrimSpace(after)
				if parts := strings.Split(after, "/"); len(parts) > 0 && parts[len(parts)-1] != "" {
					return parts[len(parts)-1]
				}
			}
		}
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return filepath.Base(abs)
	}
	return "my-service"
}

// configYAML builds a starter argos.config.yaml body. serviceName seeds
// service_name; includeSQLSection adds a commented-safe integrations.sql
// section when the project already imports database/sql.
func configYAML(serviceName string, includeSQLSection bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "service_name: %s\n", serviceName)
	b.WriteString("service_version: 0.1.0\n")
	b.WriteString("environment: local\n")
	b.WriteString("\n")
	b.WriteString("otlp_endpoint: localhost:4317\n")
	b.WriteString("otlp_protocol: grpc # grpc | http\n")
	b.WriteString("otlp_insecure: true\n")
	b.WriteString("\n")
	b.WriteString("sample_ratio: 1.0\n")
	if includeSQLSection {
		b.WriteString("\n")
		b.WriteString("integrations:\n")
		b.WriteString("  sql:\n")
		b.WriteString("    query_text: false # opt-in - query text can carry PII/secrets\n")
	}
	return b.String()
}

// mainSnippet is a ready-to-paste starter using argos.Run, printed rather
// than written to main.go directly - editing a project's existing
// entrypoint automatically is a correctness/authorization risk not worth
// taking.
func mainSnippet() string {
	return fmt.Sprintf(`Starter main.go wiring:

	err := argos.Run(context.Background(), []argos.Option{
		argos.WithYAMLConfig(%q),
	}, func(ctx context.Context, provider *argos.Provider) error {
		// register your handlers/integrations here, then serve - argos.Run
		// cancels ctx on SIGINT/SIGTERM and calls provider.Shutdown once
		// this function returns.
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
`, configFileName)
}
