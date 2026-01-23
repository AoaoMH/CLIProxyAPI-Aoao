package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

const usageSnapshotVersion = 1
const usageSnapshotRetention = 24 * time.Hour

type usageSnapshotFile struct {
	Version int                `json:"version"`
	SavedAt time.Time          `json:"saved_at"`
	Usage   StatisticsSnapshot `json:"usage"`
}

// LoadSnapshotInto merges a previously saved usage snapshot into the provided stats instance.
// If the snapshot file does not exist, it returns nil.
func LoadSnapshotInto(stats *RequestStatistics, path string) error {
	if stats == nil {
		return nil
	}
	path = stringsTrimSpace(path)
	if path == "" {
		return nil
	}
	path = filepath.Clean(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read usage snapshot: %w", err)
	}

	var payload usageSnapshotFile
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("decode usage snapshot: %w", err)
	}
	if payload.Version != 0 && payload.Version != usageSnapshotVersion {
		return fmt.Errorf("unsupported usage snapshot version: %d", payload.Version)
	}

	stats.Reset()
	stats.MergeSnapshot(payload.Usage)
	removed := stats.PruneBefore(time.Now().Add(-usageSnapshotRetention))
	// Mark as clean unless pruning removed old records (which should be persisted soon).
	stats.dirty.Store(removed)
	return nil
}

// StartSnapshotPersistence periodically saves a usage snapshot to disk when new records arrive.
// It is safe to call multiple times, but callers should generally invoke it once during startup.
func StartSnapshotPersistence(stats *RequestStatistics, path string, interval time.Duration) {
	if stats == nil {
		return
	}
	path = stringsTrimSpace(path)
	if path == "" || interval <= 0 {
		return
	}
	path = filepath.Clean(path)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			stats.PruneBefore(time.Now().Add(-usageSnapshotRetention))
			if !stats.dirty.CompareAndSwap(true, false) {
				continue
			}

			snapshot := stats.Snapshot()
			if err := saveSnapshotFile(path, snapshot); err != nil {
				stats.dirty.Store(true)
				log.WithError(err).Warn("failed to persist usage statistics snapshot")
			}
		}
	}()
}

func saveSnapshotFile(path string, snapshot StatisticsSnapshot) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create usage snapshot dir: %w", err)
	}

	payload := usageSnapshotFile{
		Version: usageSnapshotVersion,
		SavedAt: time.Now().UTC(),
		Usage:   snapshot,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode usage snapshot: %w", err)
	}

	tmp, err := os.CreateTemp(dir, "usage-snapshot-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp usage snapshot: %w", err)
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}

	if n, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp usage snapshot: %w", err)
	} else if n != len(data) {
		cleanup()
		return fmt.Errorf("write temp usage snapshot: short write (%d/%d)", n, len(data))
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp usage snapshot: %w", err)
	}
	_ = os.Chmod(tmpPath, 0o600)

	if err := os.Rename(tmpPath, path); err != nil {
		// Windows rename may fail when the destination exists.
		_ = os.Remove(path)
		if err2 := os.Rename(tmpPath, path); err2 != nil {
			cleanup()
			return fmt.Errorf("replace usage snapshot: %w", err2)
		}
	}

	return nil
}

func stringsTrimSpace(value string) string {
	if value == "" {
		return ""
	}
	// Avoid importing strings in the hot path for the logger plugin file.
	// This helper lives in persistence.go where imports are isolated.
	for len(value) > 0 {
		r := value[0]
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			break
		}
		value = value[1:]
	}
	for len(value) > 0 {
		r := value[len(value)-1]
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			break
		}
		value = value[:len(value)-1]
	}
	return value
}
