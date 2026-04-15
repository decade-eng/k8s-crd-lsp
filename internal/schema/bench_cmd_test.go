package schema_test

import (
    "fmt"
    "os"
    "testing"
    "time"

    "github.com/decade-eng/k8s-crd-lsp/internal/schema"
)

func TestBenchCompile(t *testing.T) {
    for _, fixture := range []struct{ name, path, apiPath string }{
        {"apps/v1", "../../testdata/openapi_v3_apps_v1.json", "apis/apps/v1"},
        {"core/v1", "../../testdata/openapi_v3_core_v1.json", "api/v1"},
        {"argoproj", "../../testdata/openapi_v3_argoproj_v1alpha1.json", "apis/argoproj.io/v1alpha1"},
    } {
        raw, err := os.ReadFile(fixture.path)
        if err != nil {
            t.Skipf("no fixture: %s", fixture.path)
        }
        
        t0 := time.Now()
        schemas, _ := schema.ParseAPIGroupSchemas(raw, fixture.apiPath)
        parseTime := time.Since(t0)
        
        reg := schema.NewRegistry()
        t1 := time.Now()
        reg.Load(schemas)
        compileTime := time.Since(t1)
        
        fmt.Printf("%s: parse=%v compile=%v total=%v (schemas=%d, bytes=%d)\n",
            fixture.name, parseTime, compileTime, parseTime+compileTime, len(schemas), len(raw))
    }
}
