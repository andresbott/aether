package tasks

import (
	"encoding/json"
	"testing"
)

func TestScanParamsJSONShape(t *testing.T) {
	b, err := json.Marshal(ScanParams{Full: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `{"full":true}` {
		t.Fatalf("ScanParams JSON = %s, want {\"full\":true}", b)
	}
}

func TestAvailableTasksHasSingleScan(t *testing.T) {
	scan, full := 0, 0
	for _, d := range AvailableTasks {
		switch d.ID {
		case "scan":
			scan++
		case "scan-full":
			full++
		}
	}
	if scan != 1 || full != 0 {
		t.Fatalf("scan=%d scan-full=%d, want 1 and 0", scan, full)
	}
}
