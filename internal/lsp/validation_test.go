package lsp_test

import (
	"os"
	"testing"

	"github.com/decade-eng/k8s-crd-lsp/internal/lsp"
	"github.com/decade-eng/k8s-crd-lsp/internal/schema"
	myyaml "github.com/decade-eng/k8s-crd-lsp/internal/yaml"
)

func buildValidationRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	appsRaw, err := os.ReadFile("../../testdata/openapi_v3_apps_v1.json")
	if err != nil {
		t.Fatalf("read apps/v1: %v", err)
	}
	coreRaw, err := os.ReadFile("../../testdata/openapi_v3_core_v1.json")
	if err != nil {
		t.Fatalf("read core/v1: %v", err)
	}
	appsSchemas, err := schema.ParseAPIGroupSchemas(appsRaw, "apis/apps/v1")
	if err != nil {
		t.Fatalf("parse apps/v1: %v", err)
	}
	coreSchemas, err := schema.ParseAPIGroupSchemas(coreRaw, "api/v1")
	if err != nil {
		t.Fatalf("parse core/v1: %v", err)
	}
	reg := schema.NewRegistry()
	if err := reg.Load(append(appsSchemas, coreSchemas...)); err != nil {
		t.Fatalf("load: %v", err)
	}
	return reg
}

const validDeploymentYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:latest
`

const invalidReplicasYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: "not-a-number"
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx:latest
`

func TestValidationValidDoc(t *testing.T) {
	reg := buildValidationRegistry(t)

	docs := myyaml.ParseFile(validDeploymentYAML)
	if len(docs) == 0 {
		t.Fatal("no docs parsed")
	}

	diags := lsp.ValidateDoc(docs[0], reg)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for valid Deployment, got %d: %v", len(diags), diagMessages(diags))
	}
}

func TestValidationTypeMismatch(t *testing.T) {
	reg := buildValidationRegistry(t)

	docs := myyaml.ParseFile(invalidReplicasYAML)
	if len(docs) == 0 {
		t.Fatal("no docs parsed")
	}

	diags := lsp.ValidateDoc(docs[0], reg)
	if len(diags) == 0 {
		t.Fatal("expected diagnostics for invalid replicas type, got none")
	}

	found := false
	for _, d := range diags {
		if d.Severity == lsp.SeverityError {
			found = true
			t.Logf("diagnostic: line=%d col=%d msg=%q", d.StartLine, d.StartCol, d.Message)
		}
	}
	if !found {
		t.Error("expected at least one error severity diagnostic")
	}
}

func TestValidationMissingKind(t *testing.T) {
	reg := buildValidationRegistry(t)

	yamlStr := `apiVersion: apps/v1
metadata:
  name: test
`
	docs := myyaml.ParseFile(yamlStr)
	if len(docs) == 0 {
		t.Fatal("no docs")
	}

	diags := lsp.ValidateDoc(docs[0], reg)
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for missing kind")
	}
	if diags[0].Severity != lsp.SeverityWarning {
		t.Errorf("expected Warning severity, got %d", diags[0].Severity)
	}
}

func TestValidationUnknownKind(t *testing.T) {
	reg := buildValidationRegistry(t)

	yamlStr := `apiVersion: custom.io/v1
kind: FooBarResource
metadata:
  name: test
`
	docs := myyaml.ParseFile(yamlStr)
	if len(docs) == 0 {
		t.Fatal("no docs")
	}

	diags := lsp.ValidateDoc(docs[0], reg)
	if len(diags) == 0 {
		t.Fatal("expected diagnostic for unknown kind")
	}
	if diags[0].Severity != lsp.SeverityInfo {
		t.Errorf("expected Info severity for unknown kind, got %d", diags[0].Severity)
	}
}

func TestValidationMultiDoc(t *testing.T) {
	reg := buildValidationRegistry(t)

	multiDoc := validDeploymentYAML + "---\n" + invalidReplicasYAML
	docs := myyaml.ParseFile(multiDoc)
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d", len(docs))
	}

	diags0 := lsp.ValidateDoc(docs[0], reg)
	if len(diags0) != 0 {
		t.Errorf("doc[0]: expected 0 diagnostics, got %d: %v", len(diags0), diagMessages(diags0))
	}

	diags1 := lsp.ValidateDoc(docs[1], reg)
	if len(diags1) == 0 {
		t.Fatal("doc[1]: expected diagnostics for invalid replicas")
	}

	for _, d := range diags1 {
		if d.StartLine < docs[1].LineOffset {
			t.Errorf("doc[1] diagnostic at line %d is before doc[1] start line %d",
				d.StartLine, docs[1].LineOffset)
		}
	}
	t.Logf("doc[0] diagnostics: %d, doc[1] diagnostics: %d", len(diags0), len(diags1))
}

func TestValidationNilDoc(t *testing.T) {
	reg := buildValidationRegistry(t)
	diags := lsp.ValidateDoc(nil, reg)
	if diags != nil {
		t.Error("expected nil diagnostics for nil doc")
	}
}

func TestValidationNoKindNoApiVersion(t *testing.T) {
	reg := buildValidationRegistry(t)

	yamlStr := `name: just-a-config
value: 123
`
	docs := myyaml.ParseFile(yamlStr)
	if len(docs) == 0 {
		t.Fatal("no docs")
	}

	diags := lsp.ValidateDoc(docs[0], reg)
	if len(diags) != 0 {
		t.Errorf("expected no diagnostics for non-K8s YAML, got %v", diagMessages(diags))
	}
}

func diagMessages(diags []lsp.Diagnostic) []string {
	msgs := make([]string, len(diags))
	for i, d := range diags {
		msgs[i] = d.Message
	}
	return msgs
}
