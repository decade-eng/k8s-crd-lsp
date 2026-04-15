package kubectl_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/decade-eng/k8s-crd-lsp/internal/kubectl"
)

func TestKubectlNotFound(t *testing.T) {
	e := kubectl.New("/nonexistent/path/to/kubectl")
	_, err := e.Run("version")
	if err == nil {
		t.Fatal("expected error for nonexistent kubectl path")
	}
}

func TestKubectlRun(t *testing.T) {
	tmpDir := t.TempDir()
	fakeKubectl := filepath.Join(tmpDir, "kubectl")

	script := "#!/bin/sh\necho '{\"mock\": \"response\"}'\n"
	os.WriteFile(fakeKubectl, []byte(script), 0755)

	e := kubectl.New(fakeKubectl)
	out, err := e.Run("get", "--raw", "/openapi/v3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "{\"mock\": \"response\"}\n" {
		t.Errorf("unexpected output: %q", string(out))
	}
}

func TestKubectlNonZeroExit(t *testing.T) {
	tmpDir := t.TempDir()
	fakeKubectl := filepath.Join(tmpDir, "kubectl")
	script := "#!/bin/sh\necho \"error: connection refused\" >&2\nexit 1\n"
	os.WriteFile(fakeKubectl, []byte(script), 0755)

	e := kubectl.New(fakeKubectl)
	_, err := e.Run("get", "--raw", "/openapi/v3")
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}
