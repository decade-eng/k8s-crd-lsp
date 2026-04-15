package lsp_test

import (
	"os"
	"testing"

	"github.com/decade-eng/k8s-crd-lsp/internal/lsp"
	"github.com/decade-eng/k8s-crd-lsp/internal/schema"
	myyaml "github.com/decade-eng/k8s-crd-lsp/internal/yaml"
)

func buildTestRegistry(t *testing.T) *schema.Registry {
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

func TestCompletionEmptyDoc(t *testing.T) {
	reg := buildTestRegistry(t)

	items := lsp.Provide(nil, 0, 0, reg)

	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}

	for _, expected := range []string{"apiVersion", "kind", "metadata", "spec"} {
		if !labels[expected] {
			t.Errorf("expected %q in root completions, got %v", expected, labelList(items))
		}
	}
}

func TestCompletionNoRegistry(t *testing.T) {
	items := lsp.Provide(nil, 0, 0, nil)
	if len(items) == 0 {
		t.Error("expected at least some completions even with nil registry")
	}
}

func TestCompletionKindSet(t *testing.T) {
	reg := buildTestRegistry(t)

	doc := &myyaml.Document{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	}

	items := lsp.Provide(doc, 0, 0, reg)
	if len(items) == 0 {
		t.Error("expected items for document with kind+apiVersion")
	}
}

func TestCompletionSpecLevel(t *testing.T) {
	reg := buildTestRegistry(t)

	info := reg.PropertiesAtPath("Deployment", "apps/v1", []string{"spec"})
	if info == nil {
		t.Fatal("expected PropertiesAtPath to return info for Deployment/spec")
	}

	propNames := make(map[string]bool)
	for _, p := range info.Properties {
		propNames[p.Name] = true
	}

	for _, expected := range []string{"replicas", "selector", "template"} {
		if !propNames[expected] {
			t.Errorf("expected %q in Deployment.spec properties", expected)
		}
	}
}

func TestCompletionEnumValues(t *testing.T) {
	reg := buildTestRegistry(t)

	info := reg.PropertiesAtPath("Service", "v1", []string{"spec"})
	if info == nil {
		t.Fatal("expected Service/spec info")
	}

	var typeEnums []string
	for _, p := range info.Properties {
		if p.Name == "type" {
			typeEnums = p.Enum
		}
	}

	if len(typeEnums) == 0 {
		t.Fatal("expected enum values for Service.spec.type")
	}

	enumSet := make(map[string]bool)
	for _, e := range typeEnums {
		enumSet[e] = true
	}
	for _, expected := range []string{"ClusterIP", "NodePort", "LoadBalancer"} {
		if !enumSet[expected] {
			t.Errorf("expected %q in Service.spec.type enum", expected)
		}
	}
}

func TestCompletionAllKinds(t *testing.T) {
	reg := buildTestRegistry(t)

	kinds := reg.AllKinds()
	if len(kinds) == 0 {
		t.Fatal("expected kinds from registry")
	}

	docNoKind := &myyaml.Document{Kind: "", APIVersion: ""}
	items := lsp.Provide(docNoKind, 0, 0, reg)
	if len(items) == 0 {
		t.Fatal("expected items for no-kind document")
	}

	labels := make(map[string]bool)
	for _, item := range items {
		labels[item.Label] = true
	}
	if !labels["kind"] {
		t.Error("expected 'kind' in completions for no-kind doc")
	}
}

func labelList(items []lsp.CompletionItem) []string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.Label
	}
	return labels
}
