package security

import (
	"os"
	"strings"
	"testing"
)

func TestCIWorkflowRunsTestsAndBuildOnPushAndPR(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}
	workflow := string(b)

	required := []string{
		"name: CI",
		"pull_request:",
		"push:",
		"- main",
		"actions/checkout",
		"actions/setup-go",
		"go-version-file: go.mod",
		"go test ./...",
		"make build PLATFORM=linux/amd64",
	}
	for _, needle := range required {
		if !strings.Contains(workflow, needle) {
			t.Fatalf("ci workflow missing %q", needle)
		}
	}
}
