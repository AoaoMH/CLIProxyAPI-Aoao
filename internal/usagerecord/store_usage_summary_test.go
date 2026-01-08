package usagerecord

import (
	"context"
	"testing"
)

func TestGetUsageSummary_EmptyDB(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	summary, err := store.GetUsageSummary(context.Background(), "", "")
	if err != nil {
		t.Fatalf("GetUsageSummary() error = %v", err)
	}
	if summary.TotalRequests != 0 {
		t.Fatalf("TotalRequests = %d, want 0", summary.TotalRequests)
	}
	if summary.SuccessRequests != 0 {
		t.Fatalf("SuccessRequests = %d, want 0", summary.SuccessRequests)
	}
	if summary.FailureRequests != 0 {
		t.Fatalf("FailureRequests = %d, want 0", summary.FailureRequests)
	}
	if summary.SuccessRate != 0 {
		t.Fatalf("SuccessRate = %f, want 0", summary.SuccessRate)
	}
}
