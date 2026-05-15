package targets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadPreservesOrderedTargetsPerSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save("%1", []string{"%3", "%2"}); err != nil {
		t.Fatal(err)
	}
	if err := Save("%9", []string{"%8"}); err != nil {
		t.Fatal(err)
	}

	got, err := Load("%1", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"%3", "%2"}
	if !equalStrings(got, want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestLoadFiltersStaleTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save("%1", []string{"%2", "%3", "%4"}); err != nil {
		t.Fatal(err)
	}

	got, err := Load("%1", map[string]struct{}{
		"%2": {},
		"%4": {},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"%2", "%4"}
	if !equalStrings(got, want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
}

func TestSaveEmptyClearsSourceTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Save("%1", []string{"%2"}); err != nil {
		t.Fatal(err)
	}
	if err := Save("%1", nil); err != nil {
		t.Fatal(err)
	}

	got, err := Load("%1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("targets after clear = %#v, want empty", got)
	}
}

func TestLoadCorruptStateReturnsErrorAndPreservesFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".local", "state", "tmcmt", "targets.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load("%1", nil); err == nil {
		t.Fatal("expected corrupt state error")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{not-json" {
		t.Fatalf("state file changed to %q", string(data))
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
