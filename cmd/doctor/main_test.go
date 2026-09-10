package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_ReportsSuggestionsForDetectedStdlib(t *testing.T) {
	dir := writeTempModule(t, `package main

import "net/smtp"

func main() {
	_ = smtp.PlainAuth
}
`)

	var buf bytes.Buffer
	n, err := run(dir, &buf)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if n == 0 {
		t.Fatal("expected at least one suggestion (net/smtp, missing Argos core)")
	}

	out := buf.String()
	if !strings.Contains(out, "net/smtp detected") {
		t.Errorf("expected output to mention net/smtp, got:\n%s", out)
	}
	if !strings.Contains(out, "Argos core detected") {
		t.Errorf("expected output to mention Argos core, got:\n%s", out)
	}
}

func TestRun_NonexistentDirReturnsError(t *testing.T) {
	var buf bytes.Buffer
	if _, err := run("this-directory-does-not-exist", &buf); err == nil {
		t.Fatal("expected an error for a nonexistent directory")
	}
}
