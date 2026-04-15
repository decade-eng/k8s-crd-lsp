package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Preprocess takes a raw K8s OpenAPI v3 document and returns a JSON Schema bundle
// with nullable handling, x-kubernetes-* stripping, $ref rewriting, and GVK enum injection.
//
// Output format:
//
//	{
//	  "$schema": "https://json-schema.org/draft-07/schema",
//	  "$defs": { "<schema-name>": { ...processed schema... }, ... }
//	}
//
// $refs are rewritten from "#/components/schemas/X" to "#/$defs/X".
func Preprocess(raw []byte) ([]byte, error) {
	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse openapi doc: %w", err)
	}

	components, ok := doc["components"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing components in document")
	}

	rawSchemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing components.schemas in document")
	}

	defs := make(map[string]interface{}, len(rawSchemas))
	for name, s := range rawSchemas {
		sub, ok := s.(map[string]interface{})
		if !ok {
			defs[name] = s
			continue
		}
		processed := processSchema(sub)
		injectGVKEnums(processed)
		defs[name] = processed
	}

	bundle := map[string]interface{}{
		"$schema": "https://json-schema.org/draft-07/schema",
		"$defs":   defs,
	}

	return json.Marshal(bundle)
}

func processSchema(schema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(schema))

	preserveUnknown := false
	intOrString := false

	for k, v := range schema {
		switch k {
		case "x-kubernetes-preserve-unknown-fields":
			if b, ok := v.(bool); ok && b {
				preserveUnknown = true
			}
			continue
		case "x-kubernetes-int-or-string":
			if b, ok := v.(bool); ok && b {
				intOrString = true
			}
			continue
		case "x-kubernetes-group-version-kind":
			result[k] = v
		default:
			if strings.HasPrefix(k, "x-kubernetes-") {
				continue
			}
			result[k] = v
		}
	}

	if preserveUnknown {
		delete(result, "type")
		delete(result, "properties")
		delete(result, "required")
		delete(result, "additionalProperties")
	}

	if intOrString {
		delete(result, "allOf")
		delete(result, "anyOf")
		result["type"] = []interface{}{"integer", "string"}
	}

	// Convert nullable:true to type array with "null"
	if nullable, _ := result["nullable"].(bool); nullable {
		delete(result, "nullable")
		switch t := result["type"].(type) {
		case string:
			result["type"] = []interface{}{t, "null"}
		case []interface{}:
			result["type"] = append(t, "null")
		default:
			// No type present — mark as nullable via type array
			result["type"] = []interface{}{"null"}
		}
	} else {
		delete(result, "nullable")
	}

	// Rewrite $ref from #/components/schemas/X to #/$defs/X
	if ref, ok := result["$ref"].(string); ok {
		if after, found := strings.CutPrefix(ref, "#/components/schemas/"); found {
			result["$ref"] = "#/$defs/" + after
		}
	}

	if props, ok := result["properties"].(map[string]interface{}); ok {
		newProps := make(map[string]interface{}, len(props))
		for k, v := range props {
			if sub, ok := v.(map[string]interface{}); ok {
				newProps[k] = processSchema(sub)
			} else {
				newProps[k] = v
			}
		}
		result["properties"] = newProps
	}

	if items, ok := result["items"].(map[string]interface{}); ok {
		result["items"] = processSchema(items)
	}

	if ap, ok := result["additionalProperties"].(map[string]interface{}); ok {
		result["additionalProperties"] = processSchema(ap)
	}

	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if arr, ok := result[key].([]interface{}); ok {
			out := make([]interface{}, len(arr))
			for i, item := range arr {
				if sub, ok := item.(map[string]interface{}); ok {
					out[i] = processSchema(sub)
				} else {
					out[i] = item
				}
			}
			result[key] = out
		}
	}

	return result
}

func injectGVKEnums(schema map[string]interface{}) {
	gvkRaw, ok := schema["x-kubernetes-group-version-kind"]
	if !ok {
		return
	}
	gvkList, ok := gvkRaw.([]interface{})
	if !ok || len(gvkList) == 0 {
		return
	}
	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		return
	}

	var kinds, apiVersions []interface{}
	for _, item := range gvkList {
		gvk, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		kind, _ := gvk["kind"].(string)
		group, _ := gvk["group"].(string)
		version, _ := gvk["version"].(string)

		if kind != "" {
			kinds = append(kinds, kind)
		}
		if version != "" {
			av := version
			if group != "" {
				av = group + "/" + version
			}
			apiVersions = append(apiVersions, av)
		}
	}

	if len(kinds) > 0 {
		if kp, ok := props["kind"].(map[string]interface{}); ok {
			kp["enum"] = kinds
			props["kind"] = kp
		}
	}
	if len(apiVersions) > 0 {
		if ap, ok := props["apiVersion"].(map[string]interface{}); ok {
			ap["enum"] = apiVersions
			props["apiVersion"] = ap
		}
	}
}
