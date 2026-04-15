package schema_test

import (
	"os"
	"testing"

	myschema "github.com/decade-eng/k8s-crd-lsp/internal/schema"
)

type mockKubectl struct {
	discoveryFile string
	schemaFiles   map[string]string
}

func (m *mockKubectl) Run(args ...string) ([]byte, error) {
	if len(args) == 2 && args[0] == "config" && args[1] == "current-context" {
		return []byte("test-context\n"), nil
	}
	if len(args) == 3 && args[0] == "get" && args[1] == "--raw" && args[2] == "/openapi/v3" {
		return os.ReadFile(m.discoveryFile)
	}
	if len(args) == 3 && args[0] == "get" && args[1] == "--raw" {
		url := args[2]
		if file, ok := m.schemaFiles[url]; ok {
			return os.ReadFile(file)
		}
	}
	return []byte("{}"), nil
}

func TestFetchContext(t *testing.T) {
	mock := &mockKubectl{}
	ctx, err := myschema.FetchContext(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "test-context" {
		t.Errorf("expected test-context, got %q", ctx)
	}
}

func TestFetchDiscovery(t *testing.T) {
	mock := &mockKubectl{
		discoveryFile: "../../testdata/openapi_v3_discovery.json",
	}
	result, err := myschema.FetchDiscovery(mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Paths) == 0 {
		t.Fatal("expected non-empty paths")
	}
	if _, ok := result.Paths["apis/apps/v1"]; !ok {
		t.Errorf("expected apis/apps/v1 in paths, got keys: %v", keys(result.Paths))
	}
}

func TestParseAPIGroupSchemas(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/openapi_v3_apps_v1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	schemas, err := myschema.ParseAPIGroupSchemas(raw, "apis/apps/v1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(schemas) == 0 {
		t.Fatal("expected at least one schema")
	}

	var dep *myschema.ResourceSchema
	for i, s := range schemas {
		if s.GVK.Kind == "Deployment" && s.GVK.Group == "apps" && s.GVK.Version == "v1" {
			dep = &schemas[i]
			break
		}
	}
	if dep == nil {
		t.Fatal("expected Deployment schema")
	}

	if dep.BundleURL != "file:///k8s/apis-apps-v1.json" {
		t.Errorf("unexpected BundleURL: %q", dep.BundleURL)
	}
	if dep.SchemaRef == "" {
		t.Error("expected non-empty SchemaRef")
	}
}

func TestEnumInjection(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/openapi_v3_apps_v1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	schemas, err := myschema.ParseAPIGroupSchemas(raw, "apis/apps/v1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	for _, s := range schemas {
		if s.GVK.Kind == "Deployment" {
			if len(s.BundleJSON) == 0 {
				t.Error("expected non-empty BundleJSON")
			}
			return
		}
	}
	t.Fatal("Deployment not found")
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
