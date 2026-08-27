package task

import "testing"

func TestRenderBranch(t *testing.T) {
	tests := []struct {
		name     string
		template string
		taskID   string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderBranch(tt.template, tt.taskID, tt.branch)
			if got != tt.want {
				t.Fatalf("RenderBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWorktreePath(t *testing.T) {
	got := WorktreePath("/Users/dev/Work/company/tasks", "CRO-001-payment", "project-a")
	want := "/Users/dev/Work/company/tasks/CRO-001-payment/project-a"
	if got != want {
		t.Fatalf("WorktreePath() = %q, want %q", got, want)
	}
}
