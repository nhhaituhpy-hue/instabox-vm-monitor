package githubci

import (
	"strings"
	"testing"
)

func TestBoxWorkflowYAML(t *testing.T) {
	yaml := BoxWorkflowYAML(460)
	for _, want := range []string{"runs-on: instabox-medium", "workflow_dispatch", "timeout-minutes: 470"} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("workflow yaml missing %q:\n%s", want, yaml)
		}
	}
	// zero/negative input falls back to the default session budget
	def := BoxWorkflowYAML(0)
	if !strings.Contains(def, "timeout-minutes: 470") {
		t.Fatal("default budget should be 460 minutes (470 timeout)")
	}
}

func TestOwnerFromRepo(t *testing.T) {
	owner, repo, ok := OwnerFromRepo("nhhaituhpy-hue/ci-box")
	if !ok || owner != "nhhaituhpy-hue" || repo != "ci-box" {
		t.Fatalf("got %q %q %v", owner, repo, ok)
	}
	for _, bad := range []string{"", "noslash", "a/b/c", "/repo", "owner/"} {
		if _, _, ok := OwnerFromRepo(bad); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}
