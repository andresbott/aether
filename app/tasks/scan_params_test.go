package tasks

import "testing"

// scan and scan-full are registered as two distinct tasks (each its own
// coalescing bucket) so a full scan is never dropped in favour of an in-flight
// incremental one.
func TestAvailableTasksHasScanAndScanFull(t *testing.T) {
	scan, full := 0, 0
	for _, d := range AvailableTasks {
		switch d.ID {
		case ScanTaskName:
			scan++
		case ScanFullTaskName:
			full++
		}
	}
	if scan != 1 || full != 1 {
		t.Fatalf("scan=%d scan-full=%d, want 1 and 1", scan, full)
	}
}

// reindex is enqueued by the metadata editor's write handlers, not run on
// demand, so it must not appear in the user-facing task catalogue (no trigger,
// no schedule). It stays registered on the runner independently.
func TestReindexIsNotUserExposed(t *testing.T) {
	if TaskNameExists(ReindexTaskName) {
		t.Fatalf("reindex must not be user-triggerable/schedulable")
	}
}
