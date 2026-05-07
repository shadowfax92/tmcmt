package draft

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestArchiveMovesDraftToDoneWithIncrementingFilename(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := Append("%42", "first note", "selected output"); err != nil {
		t.Fatal(err)
	}
	draftPath, exists := PathIfExists("%42")
	if !exists {
		t.Fatal("expected draft to exist")
	}

	archived, err := Archive("%42")
	if err != nil {
		t.Fatal(err)
	}

	wantPath := filepath.Join(home, ".local", "state", "tmcmt", "drafts", "done", "42-000001.md")
	if archived != wantPath {
		t.Fatalf("archive path = %q, want %q", archived, wantPath)
	}
	if _, err := os.Stat(draftPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("draft path after archive: %v", err)
	}
	if _, exists := PathIfExists("%42"); exists {
		t.Fatal("expected active draft to be gone")
	}

	data, err := os.ReadFile(archived)
	if err != nil {
		t.Fatal(err)
	}
	wantContent := "first note\n\n```\nselected output\n```\n\n"
	if string(data) != wantContent {
		t.Fatalf("archived content = %q, want %q", string(data), wantContent)
	}

	if err := Append("%42", "second note", "more output"); err != nil {
		t.Fatal(err)
	}
	archived, err = Archive("%42")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(archived) != "42-000002.md" {
		t.Fatalf("second archive basename = %q", filepath.Base(archived))
	}
}

func TestArchivePrunesDoneToRetentionLimit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	done := filepath.Join(home, ".local", "state", "tmcmt", "drafts", "done")
	if err := os.MkdirAll(done, 0o755); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 1000; i++ {
		path := filepath.Join(done, fmt.Sprintf("old-%06d.md", i))
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		ts := time.Unix(int64(i+2), 0)
		if err := os.Chtimes(path, ts, ts); err != nil {
			t.Fatal(err)
		}
	}

	if err := Append("%7", "new", "payload"); err != nil {
		t.Fatal(err)
	}
	draftPath, exists := PathIfExists("%7")
	if !exists {
		t.Fatal("expected draft to exist")
	}
	ts := time.Unix(1, 0)
	if err := os.Chtimes(draftPath, ts, ts); err != nil {
		t.Fatal(err)
	}
	archived, err := Archive("%7")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(done)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1000 {
		t.Fatalf("archive count = %d, want 1000", len(entries))
	}
	if _, err := os.Stat(filepath.Join(done, "old-000000.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("oldest archive still exists: %v", err)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("new archive missing: %v", err)
	}
}

func TestListArchivesReturnsMostRecentDoneFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	done := filepath.Join(home, ".local", "state", "tmcmt", "drafts", "done")
	if err := os.MkdirAll(done, 0o755); err != nil {
		t.Fatal(err)
	}

	older := filepath.Join(done, "42-000001.md")
	if err := os.WriteFile(older, []byte("older"), 0o644); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(done, "42-000002.md")
	if err := os.WriteFile(newer, []byte("newer"), 0o644); err != nil {
		t.Fatal(err)
	}
	sameTimeLaterName := filepath.Join(done, "99-000001.md")
	if err := os.WriteFile(sameTimeLaterName, []byte("same time later name"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldTime := time.Unix(10, 0)
	newTime := time.Unix(20, 0)
	for _, path := range []string{newer, sameTimeLaterName} {
		if err := os.Chtimes(path, newTime, newTime); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}

	archives, err := ListArchives(2)
	if err != nil {
		t.Fatal(err)
	}

	if len(archives) != 2 {
		t.Fatalf("archive count = %d, want 2", len(archives))
	}
	if archives[0].Path != sameTimeLaterName {
		t.Fatalf("first archive path = %q, want %q", archives[0].Path, sameTimeLaterName)
	}
	if archives[1].Path != newer {
		t.Fatalf("second archive path = %q, want %q", archives[1].Path, newer)
	}
	if archives[0].Size != int64(len("same time later name")) {
		t.Fatalf("first archive size = %d, want %d", archives[0].Size, len("same time later name"))
	}
}

func TestListArchivesRejectsNonPositiveLimit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := ListArchives(0); err == nil {
		t.Fatal("expected error for zero limit")
	}
}
