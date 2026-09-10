package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/jhonsferg/argos"
)

func TestRunInit_WritesConfigAndPrintsSnippet(t *testing.T) {
	dir := writeTempModule(t, `package main

import "database/sql"

func main() {
	var _ *sql.DB
}
`)

	var buf bytes.Buffer
	if err := runInit(dir, false, &buf); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	configPath := filepath.Join(dir, configFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected %s to be written: %v", configPath, err)
	}

	cfg, err := core.FromYAML(configPath)
	if err != nil {
		t.Fatalf("generated config does not parse via core.FromYAML: %v", err)
	}
	if cfg.ServiceName != "doctortestfixture" {
		t.Errorf("ServiceName = %q, want %q (from go.mod's module line)", cfg.ServiceName, "doctortestfixture")
	}

	if !bytes.Contains(data, []byte("integrations:")) {
		t.Error("expected an integrations: section since database/sql was detected")
	}

	out := buf.String()
	if !strings.Contains(out, "argos.Run") {
		t.Errorf("expected the printed snippet to mention argos.Run, got:\n%s", out)
	}
	if !strings.Contains(out, "Argos core detected") {
		t.Errorf("expected the printed suggestions to mention missing Argos core, got:\n%s", out)
	}
}

func TestRunInit_RefusesToOverwriteWithoutForce(t *testing.T) {
	dir := writeTempModule(t, "package main\n\nfunc main() {}\n")

	var buf bytes.Buffer
	if err := runInit(dir, false, &buf); err != nil {
		t.Fatalf("first runInit: %v", err)
	}

	if err := runInit(dir, false, &buf); err == nil {
		t.Fatal("expected an error on the second runInit without -force")
	}
}

func TestRunInit_ForceOverwrites(t *testing.T) {
	dir := writeTempModule(t, "package main\n\nfunc main() {}\n")

	var buf bytes.Buffer
	if err := runInit(dir, false, &buf); err != nil {
		t.Fatalf("first runInit: %v", err)
	}
	if err := runInit(dir, true, &buf); err != nil {
		t.Fatalf("second runInit with force: %v", err)
	}
}

func TestModuleName_FallsBackWithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	if got := moduleName(dir); got == "" {
		t.Error("moduleName returned empty for a directory with no go.mod")
	}
}
