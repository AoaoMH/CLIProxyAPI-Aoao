package usagerecord

import (
	"context"
	"testing"
	"time"
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

func TestGetActivityHeatmap_TimezoneTimestampFormats(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	// Insert a record with a timestamp string format that SQLite's date() may not parse.
	// We want the heatmap query to remain robust.
	now := time.Now().UTC()
	rec := &Record{
		RequestID:     "req-1",
		Timestamp:     now,
		IP:            "127.0.0.1",
		APIKey:        "k",
		APIKeyMasked:  "k",
		Model:         "m",
		Provider:      "p",
		IsStreaming:   false,
		InputTokens:   1,
		OutputTokens:  1,
		TotalTokens:   2,
		DurationMs:    10,
		StatusCode:    200,
		Success:       true,
		RequestURL:    "/v1/test",
		RequestMethod: "POST",
		RequestHeaders: map[string]string{
			"Content-Type": "application/json",
		},
		ResponseHeaders: map[string]string{
			"Content-Type": "application/json",
		},
	}
	if err := store.Insert(context.Background(), rec); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	heatmap, err := store.GetActivityHeatmap(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetActivityHeatmap() error = %v", err)
	}
	if heatmap == nil {
		t.Fatalf("heatmap is nil")
	}
	if len(heatmap.Days) != 1 {
		t.Fatalf("Days len = %d, want 1", len(heatmap.Days))
	}
	if heatmap.Days[0].Requests != 1 {
		t.Fatalf("Requests = %d, want 1", heatmap.Days[0].Requests)
	}
}
