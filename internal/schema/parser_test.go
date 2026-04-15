package schema_test

import (
	"os"
	"testing"

	myschema "github.com/decade-eng/k8s-crd-lsp/internal/schema"
)

func TestParseArgoprojCRDs(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/openapi_v3_argoproj_v1alpha1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	schemas, err := myschema.ParseAPIGroupSchemas(raw, "apis/argoproj.io/v1alpha1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(schemas) == 0 {
		t.Fatal("expected at least one CRD schema from argoproj")
	}

	for _, s := range schemas {
		if s.GVK.Kind == "" {
			t.Errorf("schema has empty kind: %+v", s.GVK)
		}
		if s.GVK.Version == "" {
			t.Errorf("schema %s has empty version", s.GVK.Kind)
		}
	}

	t.Logf("Found %d CRD schemas in argoproj: %v", len(schemas), gvkNames(schemas))
}

func TestPathToBundleURL(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/openapi_v3_apps_v1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	schemas, err := myschema.ParseAPIGroupSchemas(raw, "apis/apps/v1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, s := range schemas {
		if s.BundleURL != "file:///k8s/apis-apps-v1.json" {
			t.Errorf("expected file:///k8s/apis-apps-v1.json, got %q", s.BundleURL)
		}
	}
}

func gvkNames(schemas []myschema.ResourceSchema) []string {
	out := make([]string, len(schemas))
	for i, s := range schemas {
		out[i] = s.GVK.Kind
	}
	return out
}
