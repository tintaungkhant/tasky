package task

import "testing"

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
