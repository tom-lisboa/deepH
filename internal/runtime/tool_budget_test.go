package runtime

import (
	"strings"
	"testing"
	"time"
)

func TestTaskToolBudgetBlocksByCallCount(t *testing.T) {
	b := newTaskToolBudget(map[string]string{"tool_max_calls": "2"})
	if err := b.BeforeCall("file_read"); err != nil {
		t.Fatalf("unexpected err on call 1: %v", err)
	}
	if err := b.BeforeCall("file_read"); err != nil {
		t.Fatalf("unexpected err on call 2: %v", err)
	}
	err := b.BeforeCall("file_read")
	if err == nil || !strings.Contains(err.Error(), "tool_max_calls exceeded") {
		t.Fatalf("expected tool_max_calls exceeded, got %v", err)
	}
}

func TestTaskToolBudgetBlocksByExecTime(t *testing.T) {
	b := newTaskToolBudget(map[string]string{"tool_max_exec_ms": "10"})
	if err := b.BeforeCall("http_request"); err != nil {
		t.Fatalf("unexpected before-call err: %v", err)
	}
	b.AfterCall(12 * time.Millisecond)
	err := b.AfterExecutionCheck("http_request")
	if err == nil || !strings.Contains(err.Error(), "tool_max_exec_ms exceeded") {
		t.Fatalf("expected tool_max_exec_ms exceeded, got %v", err)
	}
}
