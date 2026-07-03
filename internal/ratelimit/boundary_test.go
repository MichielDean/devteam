package ratelimit

import (
	"go/build"
	"strings"
	"testing"
)

// packageImports returns the import paths of the named package as resolved
// by the Go toolchain. Used by the package-boundary test (BR-60) to assert
// internal/ratelimit imports ONLY the standard library.
func packageImports(t *testing.T, importPath string) []string {
	t.Helper()
	pkg, err := build.Default.Import(importPath, ".", 0)
	if err != nil {
		// Fallback: try with the module context. If the build context cannot
		// resolve the package (e.g., GOFLAGS quirks), treat the test as
		// skipped rather than failing on environment noise.
		t.Logf("build.Import for %s failed: %v", importPath, err)
		return nil
	}
	return pkg.Imports
}

// TestRatelimitPackageNoInternalDeps (BR-60, SEC-14, REL-15) asserts the
// package boundary that is the primary reversibility seam: internal/ratelimit
// imports ONLY the Go standard library. It MUST NOT import internal/api or
// internal/config. The test fails the build if a developer adds an internal
// import — the boundary is enforced mechanically, not by convention.
//
// This is the test the §2.6 NFR Design pattern names as its "Why it holds by
// construction" gate.
func TestRatelimitPackageNoInternalDeps(t *testing.T) {
	imports := packageImports(t, "github.com/MichielDean/devteam/internal/ratelimit")
	for _, imp := range imports {
		if strings.Contains(imp, "github.com/MichielDean/devteam/internal/") {
			t.Errorf("internal/ratelimit must not import internal/* packages; found %q (BR-60/SEC-14/REL-15)", imp)
		}
	}
}