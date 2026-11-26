package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func TestSaveFileIfUnchangedConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	expected := Config{Roots: []Root{{Name: "original", Path: "/original"}}}
	if err := expected.SaveFile(path); err != nil {
		t.Fatal(err)
	}

	writers := []Config{
		{Roots: []Root{{Name: "first", Path: "/first"}, {Name: "first-extra", Path: "/first-extra"}}},
		{Roots: []Root{{Name: "second", Path: "/second"}, {Name: "second-extra", Path: "/second-extra"}}},
	}
	start := make(chan struct{})
	errs := make([]error, len(writers))
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = writers[i].SaveFileIfUnchanged(path, expected)
		}(i)
	}
	close(start)
	wg.Wait()

	winner := -1
	for i, err := range errs {
		if err == nil {
			if winner != -1 {
				t.Fatal("both writers saved from the same expected config")
			}
			winner = i
		} else if !errors.Is(err, ErrChanged) {
			// A writer that arrives while the winner holds the advisory lock may
			// fail immediately with the documented retryable lock error.
			if err.Error() != "preferences are being saved by another Tasky process; retry" {
				t.Fatalf("writer %d error = %v, want ErrChanged or lock busy", i, err)
			}
		}
	}
	if winner == -1 {
		t.Fatalf("no writer saved successfully: %v", errs)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatalf("saved YAML is incomplete or invalid: %v", err)
	}
	if !reflect.DeepEqual(got.Roots, writers[winner].Roots) {
		t.Fatalf("saved Roots = %#v, want winning config %#v", got.Roots, writers[winner].Roots)
	}
}

func TestSaveFileIfUnchangedRejectsStaleBytesAndPreservesPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	expected := Config{Roots: []Root{{Name: "before", Path: "/before"}}}
	if err := expected.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := Config{Roots: []Root{{Name: "external", Path: "/external"}}}
	if err := changed.SaveFile(path); err != nil {
		t.Fatal(err)
	}
	currentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := Config{Roots: []Root{{Name: "stale", Path: "/stale"}}}
	err = stale.SaveFileIfUnchanged(path, expected)
	if !errors.Is(err, ErrChanged) {
		t.Fatalf("stale save error = %v, want ErrChanged", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(currentBytes) {
		t.Fatal("stale save replaced the current file bytes")
	}
	if string(after) == string(before) {
		t.Fatal("setup failed: current file did not differ from stale expectation")
	}
	current, err := LoadFile(path)
	if err != nil || !reflect.DeepEqual(current.Roots, changed.Roots) {
		t.Fatalf("stale save replaced current config: got %#v, err %v", current, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("saved permissions = %#o, want 0600", got)
		}
	}
}

func TestSaveFilePreservesConfigSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.yaml")
	link := filepath.Join(dir, "config.yaml")
	initial := Config{Roots: []Root{{Name: "before", Path: "/before"}}}
	if err := initial.SaveFile(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Logf("symlink verification unavailable on this host: %v", err)
		return
	}
	want := Config{Roots: []Root{{Name: "after", Path: "/after"}}}
	if err := want.SaveFile(link); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("config symlink was replaced")
	}
	got, err := LoadFile(target)
	if err != nil || !reflect.DeepEqual(got.Roots, want.Roots) {
		t.Fatalf("target config = %#v, err %v; want %#v", got, err, want)
	}
}
