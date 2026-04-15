package schema_test

import (
	"os"
	"testing"

	myschema "github.com/decade-eng/k8s-crd-lsp/internal/schema"
)

func loadRegistry(t *testing.T) *myschema.Registry {
	t.Helper()

	appsRaw, err := os.ReadFile("../../testdata/openapi_v3_apps_v1.json")
	if err != nil {
		t.Fatalf("read apps/v1: %v", err)
	}
	coreRaw, err := os.ReadFile("../../testdata/openapi_v3_core_v1.json")
	if err != nil {
		t.Fatalf("read core/v1: %v", err)
	}

	appsSchemas, err := myschema.ParseAPIGroupSchemas(appsRaw, "apis/apps/v1")
	if err != nil {
		t.Fatalf("parse apps/v1: %v", err)
	}
	coreSchemas, err := myschema.ParseAPIGroupSchemas(coreRaw, "api/v1")
	if err != nil {
		t.Fatalf("parse core/v1: %v", err)
	}

	reg := myschema.NewRegistry()
	if err := reg.Load(append(appsSchemas, coreSchemas...)); err != nil {
		t.Fatalf("load registry: %v", err)
	}
	return reg
}

func TestRegistryLookup(t *testing.T) {
	reg := loadRegistry(t)

	dep := reg.Lookup("Deployment", "apps/v1")
	if dep == nil {
		t.Fatal("expected Deployment/apps/v1, got nil")
	}

	svc := reg.Lookup("Service", "v1")
	if svc == nil {
		t.Fatal("expected Service/v1, got nil")
	}

	unknown := reg.Lookup("NonExistentResource", "v1")
	if unknown != nil {
		t.Fatal("expected nil for unknown kind")
	}
}

func TestRegistryAllKinds(t *testing.T) {
	reg := loadRegistry(t)

	kinds := reg.AllKinds()
	if len(kinds) == 0 {
		t.Fatal("expected non-empty kinds list")
	}

	kindSet := make(map[string]bool)
	for _, k := range kinds {
		kindSet[k] = true
	}
	for _, expected := range []string{"Deployment", "Service", "Pod", "ConfigMap"} {
		if !kindSet[expected] {
			t.Errorf("expected kind %q in AllKinds", expected)
		}
	}

	for i := 1; i < len(kinds); i++ {
		if kinds[i] < kinds[i-1] {
			t.Errorf("AllKinds not sorted: %q < %q", kinds[i], kinds[i-1])
		}
	}
}

func TestRegistryAPIVersionsForKind(t *testing.T) {
	reg := loadRegistry(t)

	avs := reg.APIVersionsForKind("Deployment")
	if len(avs) == 0 {
		t.Fatal("expected apiVersions for Deployment")
	}
	found := false
	for _, av := range avs {
		if av == "apps/v1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected apps/v1 in apiVersions for Deployment, got %v", avs)
	}
}

func TestPropertiesAtPath(t *testing.T) {
	reg := loadRegistry(t)

	info := reg.PropertiesAtPath("Deployment", "apps/v1", []string{"spec"})
	if info == nil {
		t.Fatal("expected non-nil PathInfo for Deployment/spec")
	}

	propNames := make(map[string]bool)
	for _, p := range info.Properties {
		propNames[p.Name] = true
	}

	for _, expected := range []string{"replicas", "selector", "template"} {
		if !propNames[expected] {
			t.Errorf("expected property %q in Deployment.spec, got %v", expected, propNames)
		}
	}
}

func TestPropertiesAtPathEnum(t *testing.T) {
	reg := loadRegistry(t)

	info := reg.PropertiesAtPath("Service", "v1", []string{"spec"})
	if info == nil {
		t.Fatal("expected non-nil PathInfo for Service/spec")
	}

	for _, p := range info.Properties {
		if p.Name == "type" {
			if len(p.Enum) == 0 {
				t.Error("expected enum values for Service.spec.type")
			}
			enumSet := make(map[string]bool)
			for _, v := range p.Enum {
				enumSet[v] = true
			}
			for _, expected := range []string{"ClusterIP", "NodePort", "LoadBalancer"} {
				if !enumSet[expected] {
					t.Errorf("expected %q in Service.spec.type enum, got %v", expected, p.Enum)
				}
			}
			return
		}
	}
	t.Error("Service.spec.type property not found")
}

func TestPropertiesAtPathUnknown(t *testing.T) {
	reg := loadRegistry(t)

	info := reg.PropertiesAtPath("NonExistent", "v1", []string{"spec"})
	if info != nil {
		t.Error("expected nil for unknown kind")
	}

	info = reg.PropertiesAtPath("Deployment", "apps/v1", []string{"nonexistent"})
	if info != nil {
		t.Errorf("expected nil for unknown path, got %v", info)
	}
}
