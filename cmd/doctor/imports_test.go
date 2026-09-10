package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempModule creates a minimal, standalone Go module (its own go.mod,
// deliberately outside this repo's go.work) so CollectImports exercises a
// real go/packages load. Every import used here is standard library, so
// resolving it never touches the network or the module cache.
func writeTempModule(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module doctortestfixture\n\ngo 1.26.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(source), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	return dir
}

func TestCollectImports_FindsDirectStdlibImports(t *testing.T) {
	dir := writeTempModule(t, `package main

import (
	"database/sql"
	"net/http"
)

func main() {
	_ = sql.ErrNoRows
	_ = http.StatusOK
}
`)

	imports, err := CollectImports(dir)
	if err != nil {
		t.Fatalf("CollectImports: %v", err)
	}

	for _, want := range []string{"database/sql", "net/http", "doctortestfixture"} {
		if !imports[want] {
			t.Errorf("expected imports to contain %q, got %v", want, imports)
		}
	}
	if imports["net/smtp"] {
		t.Errorf("did not expect net/smtp in imports, got %v", imports)
	}
}

func TestCollectImports_InvalidDirectoryErrors(t *testing.T) {
	dir := writeTempModule(t, `package main

func main() {}
`)
	// Point at a subdirectory that doesn't exist: packages.Load reports this
	// as a package error rather than a hard Go error, so this also verifies
	// CollectImports surfaces it as an error rather than silently returning
	// an empty set.
	if _, err := CollectImports(filepath.Join(dir, "does-not-exist")); err == nil {
		t.Fatal("expected an error for a nonexistent directory")
	}
}
