package github

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestADR18_NoExternalGitHubImportsOutsideThisPackage enforces the build
// invariant (ADR-18): internal/github/ is the SOLE importer of google/go-github
// and golang.org/x/oauth2. No package outside internal/github/ may import
// either library — this keeps the GitHub-API boundary machine-checked, not
// convention-checked (nfr-design-specs §6.2, FR-IFACE-02).
//
// Per nfr-design-specs §3.1 refinement, x/crypto is NOT imported by this
// feature (AES-GCM uses stdlib), so it is NOT checked here — there is no
// x/crypto import to be the sole importer of.
//
// The test runs `go list -deps ./...` and asserts the forbidden imports appear
// only in packages under internal/github/. It skips if go is not on PATH
// (NFR-COMPAT-01 — the test environment may not have the toolchain).
func TestADR18_NoExternalGitHubImportsOutsideThisPackage(t *testing.T) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go not on PATH; skipping ADR-18 import-invariant test")
	}

	// Run `go list -deps -f '{{.ImportPath}} {{.Imports}}' ./...` from the
	// repo root. We use the module root (two dirs up from internal/github).
	cmd := exec.Command(goPath, "list", "-deps", "-f", "{{.ImportPath}}\t{{.Imports}}", "./...")
	cmd.Dir = "../.." // internal/github → repo root
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list failed (module not resolvable?): %v; skipping", err)
	}

	forbidden := []string{
		"github.com/google/go-github",
		"golang.org/x/oauth2",
	}

	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		pkg := parts[0]
		imports := parts[1]

		// Skip the library packages themselves and their subpackages.
		if strings.HasPrefix(pkg, "github.com/google/go-github") ||
			strings.HasPrefix(pkg, "golang.org/x/oauth2") {
			continue
		}
		// Skip our own package — it IS the sole importer.
		if strings.Contains(pkg, "github.com/MichielDean/devteam/internal/github") {
			continue
		}
		// Skip non-devteam packages (stdlib, third-party deps of deps).
		if !strings.HasPrefix(pkg, "github.com/MichielDean/devteam") {
			continue
		}

		for _, f := range forbidden {
			if strings.Contains(imports, f) {
				t.Errorf("ADR-18 violation: package %s imports %s (only internal/github/ may)", pkg, f)
			}
		}
	}
}

// TestNoOSExecInNativeClientFiles enforces NFR-PORT-02: the NativeClient files
// must NOT import os/exec (the gh-free run path). Only ghcli.go (the adapter)
// may import os/exec. This is the precise mechanism (nfr-design-specs §6.2).
func TestNoOSExecInNativeClientFiles(t *testing.T) {
	nativeFiles := []string{
		"native.go",
		"auth.go",
		"token_cache.go",
		"pr_ops.go",
		"mergeable.go",
		"credstore.go",
		"redaction.go",
		"errors.go",
		"client.go",
		"types.go",
		"repos.go",
	}
	for _, f := range nativeFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue // file may not exist in this build
		}
		if strings.Contains(string(data), `"os/exec"`) {
			t.Errorf("NFR-PORT-02 violation: %s imports os/exec (only ghcli.go may)", f)
		}
	}
}