package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestVersionOneMigratesWithoutChangingIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.sqlite3")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO roots(id, name, path, path_key) VALUES('stable', 'old', '/old', '/old')"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t, path)
	roots, err := s.ListRoots()
	if err != nil || len(roots) != 1 || roots[0].ID != "stable" {
		t.Fatalf("migration changed Root: %+v %v", roots, err)
	}
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 3 {
		t.Fatalf("version %d %v", version, err)
	}
	if _, err := s.ListCreations("stable"); err != nil {
		t.Fatal(err)
	}
}
