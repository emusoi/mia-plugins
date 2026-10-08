package main

import "testing"

func TestLayerNamesComeFromTheStepsHead(t *testing.T) {
	for text, want := range map[string]string{
		"Add the due_date column — and backfill it": "add-the-due-date-column",
		"Sort the task list: by date":               "sort-the-task-list",
		"  API endpoint  ":                          "api-endpoint",
	} {
		if got := layerName(text); got != want {
			t.Errorf("%q → %q, want %q", text, got, want)
		}
	}
}
