package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCatPrintsLatestDoneSessionWithFullPathHeader(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	done := filepath.Join(home, ".local", "state", "tmcmt", "drafts", "done")
	writeDoneSession(t, done, "42-000001.md", "older\n", time.Unix(10, 0))
	newer := writeDoneSession(t, done, "42-000002.md", "newer\n", time.Unix(20, 0))

	out, err := executeRootForTest(t, "cat")
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf("==> %s <==\nnewer\n", newer)
	if out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}

func TestLsAliasPrintsMultipleDoneSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	done := filepath.Join(home, ".local", "state", "tmcmt", "drafts", "done")
	older := writeDoneSession(t, done, "42-000001.md", "older\n", time.Unix(10, 0))
	newer := writeDoneSession(t, done, "42-000002.md", "newer", time.Unix(20, 0))

	out, err := executeRootForTest(t, "ls", "-n", "2")
	if err != nil {
		t.Fatal(err)
	}

	want := fmt.Sprintf("==> %s <==\nnewer\n\n==> %s <==\nolder\n", newer, older)
	if out != want {
		t.Fatalf("output = %q, want %q", out, want)
	}
}

func TestCatRejectsNonPositiveCount(t *testing.T) {
	_, err := executeRootForTest(t, "cat", "-n", "0")
	if err == nil {
		t.Fatal("expected error for zero count")
	}
}

func executeRootForTest(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	resetCatCountFlag(t)
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	t.Cleanup(func() {
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(os.Stdout)
		rootCmd.SetErr(os.Stderr)
		resetCatCountFlag(t)
	})

	err := rootCmd.Execute()
	return out.String(), err
}

func resetCatCountFlag(t *testing.T) {
	t.Helper()

	catCount = 1
	flag := catCmd.Flags().Lookup("count")
	if flag == nil {
		t.Fatal("cat count flag missing")
	}
	if err := flag.Value.Set("1"); err != nil {
		t.Fatal(err)
	}
	flag.Changed = false
}

func writeDoneSession(t *testing.T, doneDir, name, content string, modTime time.Time) string {
	t.Helper()

	if err := os.MkdirAll(doneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(doneDir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
	return path
}
