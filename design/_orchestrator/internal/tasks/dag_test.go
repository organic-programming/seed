package tasks

import (
	"strings"
	"testing"
)

func TestSort(t *testing.T) {
	entries := []Entry{
		{Number: "05", DependsOn: []string{"v0.7 TASK01"}},
		{Number: "04", DependsOn: []string{"TASK01–03"}},
		{Number: "03", DependsOn: []string{"TASK01", "TASK02"}},
		{Number: "02"},
		{Number: "01"},
	}

	ordered, err := Sort(entries)
	if err != nil {
		t.Fatalf("Sort returned error: %v", err)
	}

	if len(ordered) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(ordered))
	}

	positions := make(map[string]int, len(ordered))
	for i, entry := range ordered {
		positions[entry.Number] = i
	}

	if positions["01"] >= positions["03"] {
		t.Fatalf("TASK01 should be ordered before TASK03: %v", positions)
	}
	if positions["02"] >= positions["03"] {
		t.Fatalf("TASK02 should be ordered before TASK03: %v", positions)
	}
	if positions["01"] >= positions["04"] || positions["02"] >= positions["04"] || positions["03"] >= positions["04"] {
		t.Fatalf("TASK04 should be ordered after TASK01-TASK03: %v", positions)
	}
}

func TestSortDetectsCycle(t *testing.T) {
	entries := []Entry{
		{Number: "01", DependsOn: []string{"TASK02"}},
		{Number: "02", DependsOn: []string{"TASK01"}},
	}

	_, err := Sort(entries)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if !strings.Contains(err.Error(), "dependency cycle detected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "TASK01") || !strings.Contains(err.Error(), "TASK02") {
		t.Fatalf("cycle error should mention both tasks: %v", err)
	}
}
