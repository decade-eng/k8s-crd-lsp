package schema_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	goyaml "gopkg.in/yaml.v3"

	myschema "github.com/decade-eng/k8s-crd-lsp/internal/schema"
)

const appsV1URL = "file:///k8s/apps/v1.json"
const deploymentRef = appsV1URL + "#/$defs/io.k8s.api.apps.v1.Deployment"

func loadAndPreprocess(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	out, err := myschema.Preprocess(raw)
	if err != nil {
		t.Fatalf("preprocess %s: %v", path, err)
	}
	return out
}

func compileSchema(t *testing.T, url string, data []byte) *jsonschema.Compiler {
	t.Helper()
	c := jsonschema.NewCompiler()
	if err := c.AddResource(url, strings.NewReader(string(data))); err != nil {
		t.Fatalf("add resource: %v", err)
	}
	return c
}

func yamlToJSONVal(t *testing.T, yamlStr string) interface{} {
	t.Helper()
	var node goyaml.Node
	if err := goyaml.Unmarshal([]byte(yamlStr), &node); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	var obj interface{}
	if err := node.Decode(&obj); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	var jsonVal interface{}
	if err := json.Unmarshal(jsonBytes, &jsonVal); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	return jsonVal
}

func TestSpike(t *testing.T) {
	t.Run("AppsV1SchemasLoadable", func(t *testing.T) {
		bundle := loadAndPreprocess(t, "../../testdata/openapi_v3_apps_v1.json")

		c := compileSchema(t, appsV1URL, bundle)
		_, err := c.Compile(deploymentRef)
		if err != nil {
			t.Fatalf("compile Deployment schema: %v", err)
		}
	})

	t.Run("DeploymentValidYAML", func(t *testing.T) {
		bundle := loadAndPreprocess(t, "../../testdata/openapi_v3_apps_v1.json")
		c := compileSchema(t, appsV1URL, bundle)
		s, err := c.Compile(deploymentRef)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}

		yaml := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
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
		val := yamlToJSONVal(t, yaml)
		if err := s.Validate(val); err != nil {
			t.Fatalf("valid deployment failed validation: %v", err)
		}
	})

	t.Run("DeploymentInvalidYAMLFails", func(t *testing.T) {
		bundle := loadAndPreprocess(t, "../../testdata/openapi_v3_apps_v1.json")
		c := compileSchema(t, appsV1URL, bundle)
		s, err := c.Compile(deploymentRef)
		if err != nil {
			t.Fatalf("compile: %v", err)
		}

		yaml := `
apiVersion: apps/v1
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
		val := yamlToJSONVal(t, yaml)
		err = s.Validate(val)
		if err == nil {
			t.Fatal("expected validation error for string replicas, got none")
		}

		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("expected *ValidationError, got %T: %v", err, err)
		}

		found := false
		var checkCauses func(e *jsonschema.ValidationError)
		checkCauses = func(e *jsonschema.ValidationError) {
			if strings.Contains(e.InstanceLocation, "replicas") {
				found = true
			}
			for _, c := range e.Causes {
				checkCauses(c)
			}
		}
		checkCauses(ve)

		if !found {
			t.Errorf("expected error at 'replicas', got: %v", err)
		}
	})

	t.Run("EnumInjection", func(t *testing.T) {
		bundle := loadAndPreprocess(t, "../../testdata/openapi_v3_apps_v1.json")

		var bundleMap map[string]interface{}
		if err := json.Unmarshal(bundle, &bundleMap); err != nil {
			t.Fatalf("unmarshal bundle: %v", err)
		}

		defs := bundleMap["$defs"].(map[string]interface{})
		dep := defs["io.k8s.api.apps.v1.Deployment"].(map[string]interface{})
		props := dep["properties"].(map[string]interface{})

		kindProp := props["kind"].(map[string]interface{})
		kindEnum := kindProp["enum"].([]interface{})
		if len(kindEnum) == 0 || kindEnum[0] != "Deployment" {
			t.Errorf("expected kind.enum = [Deployment], got %v", kindEnum)
		}

		avProp := props["apiVersion"].(map[string]interface{})
		avEnum := avProp["enum"].([]interface{})
		if len(avEnum) == 0 || avEnum[0] != "apps/v1" {
			t.Errorf("expected apiVersion.enum = [apps/v1], got %v", avEnum)
		}
	})

	t.Run("NullableHandled", func(t *testing.T) {
		// Synthesize a schema with nullable:true to test the preprocessing path.
		// The real K8s fixtures happen to have no nullable fields, but the preprocessor
		// must handle it correctly for older cluster versions that do.
		syntheticDoc := `{
			"openapi": "3.0.0",
			"info": {"title": "Test", "version": "v1"},
			"paths": {},
			"components": {
				"schemas": {
					"io.test.v1.Foo": {
						"type": "object",
						"nullable": true,
						"properties": {
							"bar": {"type": "string", "nullable": true}
						}
					}
				}
			}
		}`

		bundle, err := myschema.Preprocess([]byte(syntheticDoc))
		if err != nil {
			t.Fatalf("preprocess: %v", err)
		}

		var bundleMap map[string]interface{}
		if err := json.Unmarshal(bundle, &bundleMap); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		defs := bundleMap["$defs"].(map[string]interface{})
		foo := defs["io.test.v1.Foo"].(map[string]interface{})

		if _, hasNullable := foo["nullable"]; hasNullable {
			t.Error("nullable field should be removed from top-level schema")
		}

		typeVal, ok := foo["type"].([]interface{})
		if !ok {
			t.Fatalf("expected type to be array, got %T: %v", foo["type"], foo["type"])
		}
		hasNull := false
		for _, v := range typeVal {
			if v == "null" {
				hasNull = true
			}
		}
		if !hasNull {
			t.Errorf("expected null in type array, got %v", typeVal)
		}

		props := foo["properties"].(map[string]interface{})
		bar := props["bar"].(map[string]interface{})
		if _, hasNullable := bar["nullable"]; hasNullable {
			t.Error("nullable should be removed from nested property")
		}
		barType, ok := bar["type"].([]interface{})
		if !ok {
			t.Fatalf("expected bar.type to be array, got %T", bar["type"])
		}
		hasNull = false
		for _, v := range barType {
			if v == "null" {
				hasNull = true
			}
		}
		if !hasNull {
			t.Errorf("expected null in bar.type array, got %v", barType)
		}
	})
}
