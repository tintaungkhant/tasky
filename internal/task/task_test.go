package task

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderBranch(t *testing.T) {
	tests := []struct {
		name     string
		template string
		taskID   string
		taskName string
		branch   string
		want     string
	}{
		{
			name:     "both placeholders",
			template: "task/{task-id}/{branch}",
			taskID:   "CRO-001-payment",
			branch:   "feat/payment",
			want:     "task/CRO-001-payment/feat/payment",
		},
		{
			name:     "branch only",
			template: "{branch}",
			taskID:   "CRO-001-payment",
			branch:   "feat/payment",
			want:     "feat/payment",
		},
		{
			name:     "no truncation of task id",
			template: "{task-id}",
			taskID:   "CRO-001-payment-with-a-long-descriptive-name",
			branch:   "feat/payment",
			want:     "CRO-001-payment-with-a-long-descriptive-name",
		},
		{
			name:     "task name substituted",
			template: "task/{task-id}/{task-name}/{branch}",
			taskID:   "CRO-001",
			taskName: "laravel-upgrade",
			branch:   "feat/payment",
			want:     "task/CRO-001/laravel-upgrade/feat/payment",
		},
		{
			name:     "empty task name substitutes literally empty",
			template: "task/{task-id}/{task-name}/{branch}",
			taskID:   "CRO-001",
			branch:   "feat/payment",
			want:     "task/CRO-001//feat/payment",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderBranch(tt.template, tt.taskID, tt.taskName, tt.branch)
			if got != tt.want {
				t.Fatalf("RenderBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskDirName(t *testing.T) {
	if got := TaskDirName("CRO-001", ""); got != "CRO-001" {
		t.Fatalf("TaskDirName() = %q, want %q", got, "CRO-001")
	}
	if got := TaskDirName("CRO-001", "laravel-upgrade"); got != "CRO-001[laravel-upgrade]" {
		t.Fatalf("TaskDirName() = %q, want %q", got, "CRO-001[laravel-upgrade]")
	}
}

func TestParseTaskDirName(t *testing.T) {
	tests := []struct {
		dirName      string
		wantTaskID   string
		wantTaskName string
	}{
		{"CRO-001", "CRO-001", ""},
		{"CRO-001[laravel-upgrade]", "CRO-001", "laravel-upgrade"},
	}
	for _, tt := range tests {
		id, name := ParseTaskDirName(tt.dirName)
		if id != tt.wantTaskID || name != tt.wantTaskName {
			t.Fatalf("ParseTaskDirName(%q) = (%q, %q), want (%q, %q)", tt.dirName, id, name, tt.wantTaskID, tt.wantTaskName)
		}
	}
}

func TestWorktreePath(t *testing.T) {
	got := WorktreePath("/Users/dev/Work/company/tasks", "CRO-001-payment", "project-a", "feat/payment-bank-a")
	want := "/Users/dev/Work/company/tasks/CRO-001-payment/project-a[feat-payment-bank-a]"
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}

func TestWorktreePathDistinctBranchesSameRepo(t *testing.T) {
	a := WorktreePath("/tasks", "CRO-001", "project-a", "feat/payment-bank-a")
	b := WorktreePath("/tasks", "CRO-001", "project-a", "feat/payment-bank-b")
	if a == b {
		t.Fatalf("WorktreePath() collided for different branches: %q", a)
	}
}

func TestListEntries(t *testing.T) {
	tasksDir := t.TempDir()

	entries, err := ListEntries(filepath.Join(tasksDir, "does-not-exist"))
	if err != nil {
		t.Fatalf("ListEntries on missing dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("ListEntries on missing dir = %v, want empty", entries)
	}

	for _, dir := range []string{"CRO-002", "CRO-001[laravel-upgrade]"} {
		if err := os.MkdirAll(filepath.Join(tasksDir, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tasksDir, "not-a-dir"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err = ListEntries(tasksDir)
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	want := []Entry{{ID: "CRO-001", Name: "laravel-upgrade"}, {ID: "CRO-002", Name: ""}}
	if len(entries) != len(want) || entries[0] != want[0] || entries[1] != want[1] {
		t.Fatalf("ListEntries = %v, want %v", entries, want)
	}
}

func TestFindEntry(t *testing.T) {
	entries := []Entry{{ID: "CRO-001", Name: "laravel-upgrade"}, {ID: "CRO-002"}}
	if got, ok := FindEntry(entries, "CRO-002"); !ok || got.Name != "" {
		t.Fatalf("FindEntry(CRO-002) = %v, %v", got, ok)
	}
	if _, ok := FindEntry(entries, "missing"); ok {
		t.Fatal("FindEntry(missing) unexpectedly found")
	}
}

func TestEntryLabel(t *testing.T) {
	if got := (Entry{ID: "CRO-001"}).Label(); got != "CRO-001" {
		t.Fatalf("Label() = %q, want %q", got, "CRO-001")
	}
	if got := (Entry{ID: "CRO-001", Name: "laravel-upgrade"}).Label(); got != "CRO-001 (laravel-upgrade)" {
		t.Fatalf("Label() = %q, want %q", got, "CRO-001 (laravel-upgrade)")
	}
}
