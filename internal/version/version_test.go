package version

import (
	"strings"
	"testing"
)

func TestCurrentDefaults(t *testing.T) {
	got := Current()
	if got.Version != "dev" || got.Commit != "none" || got.BuildTime != "unknown" {
		t.Fatalf("Current() = %#v", got)
	}
}

func TestStringIncludesVersionFields(t *testing.T) {
	got := String()
	for _, want := range []string{"couswee dev", "commit none", "built unknown"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String() = %q, missing %q", got, want)
		}
	}
}
