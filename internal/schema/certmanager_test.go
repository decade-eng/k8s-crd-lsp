package schema_test

import (
    "os"
    "testing"

    "github.com/decade-eng/k8s-crd-lsp/internal/schema"
)

func TestCertManagerLoad(t *testing.T) {
    raw, err := os.ReadFile("../../testdata/openapi_v3_certmanager_v1.json")
    if err != nil {
        t.Skip("no cert-manager fixture")
    }

    schemas, err := schema.ParseAPIGroupSchemas(raw, "apis/cert-manager.io/v1")
    if err != nil {
        t.Fatalf("parse: %v", err)
    }
    t.Logf("Parsed %d schemas", len(schemas))

    var certFound bool
    for _, s := range schemas {
        if s.GVK.Kind == "Certificate" {
            t.Logf("Certificate: GVK=%+v BundleURL=%s SchemaRef=%s", s.GVK, s.BundleURL, s.SchemaRef)
            certFound = true
        }
    }
    if !certFound {
        t.Fatal("Certificate schema not found in parsed schemas")
    }

    reg := schema.NewRegistry()
    if err := reg.Load(schemas); err != nil {
        t.Fatalf("load: %v", err)
    }

    // Try the exact lookup the validation uses
    s := reg.Lookup("Certificate", "cert-manager.io/v1")
    if s == nil {
        t.Fatal("Certificate/cert-manager.io/v1 NOT FOUND in registry after Load")
    }
    t.Log("Certificate/cert-manager.io/v1 found in registry")
    
    // Check all kinds
    kinds := reg.AllKinds()
    t.Logf("All kinds: %v", kinds)
}
