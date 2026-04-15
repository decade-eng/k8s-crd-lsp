package schema

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ParseAPIGroupSchemas(raw []byte, apiPath string) ([]ResourceSchema, error) {
	bundleJSON, err := Preprocess(raw)
	if err != nil {
		return nil, fmt.Errorf("preprocess %s: %w", apiPath, err)
	}

	bundleURL := pathToBundleURL(apiPath)

	var bundle struct {
		Defs map[string]json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(bundleJSON, &bundle); err != nil {
		return nil, fmt.Errorf("parse bundle %s: %w", apiPath, err)
	}

	var results []ResourceSchema
	for defName, defRaw := range bundle.Defs {
		var schema struct {
			XGVK []struct {
				Group   string `json:"group"`
				Version string `json:"version"`
				Kind    string `json:"kind"`
			} `json:"x-kubernetes-group-version-kind"`
			Properties struct {
				APIVersion interface{} `json:"apiVersion"`
				Kind       interface{} `json:"kind"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(defRaw, &schema); err != nil {
			continue
		}
		if len(schema.XGVK) == 0 || schema.Properties.APIVersion == nil || schema.Properties.Kind == nil {
			continue
		}

		for _, gvk := range schema.XGVK {
			if gvk.Kind == "" || gvk.Version == "" {
				continue
			}
			results = append(results, ResourceSchema{
				GVK: GroupVersionKind{
					Group:   gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind,
				},
				BundleJSON: bundleJSON,
				BundleURL:  bundleURL,
				SchemaRef:  bundleURL + "#/$defs/" + defName,
			})
		}
	}

	return results, nil
}

func pathToBundleURL(apiPath string) string {
	safe := strings.ReplaceAll(strings.TrimPrefix(apiPath, "/"), "/", "-")
	return "file:///k8s/" + safe + ".json"
}
