//go:build !phocus

package main

import (
	"strings"
	"testing"
)

func TestStudioReleaseRequiresZeroScore(t *testing.T) {
	if err := validateStudioReleaseScore(studioEvaluation{Score: 0}); err != nil {
		t.Fatalf("zero score blocked: %v", err)
	}
	err := validateStudioReleaseScore(studioEvaluation{Score: 5, Results: []studioEvaluationResult{{Message: "Firewall enabled", Points: 5, Status: "passing"}}})
	if err == nil {
		t.Fatal("nonzero score was allowed")
	}
	if !strings.Contains(err.Error(), "exactly 0") || !strings.Contains(err.Error(), "Firewall enabled") {
		t.Fatalf("release error is not actionable: %v", err)
	}
}
