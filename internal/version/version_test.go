package version

import "testing"

func TestCurrentReturnsInjectedCommit(t *testing.T) {
	original := Commit
	Commit = "test-commit"
	t.Cleanup(func() { Commit = original })

	if got := Current(); got != "test-commit" {
		t.Fatalf("Current() = %q, want injected commit", got)
	}
}
