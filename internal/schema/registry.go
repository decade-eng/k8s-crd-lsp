package schema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

type PropertyInfo struct {
	Name     string
	Type     string
	Enum     []string
	Required bool
	IsArray  bool
}

type PathInfo struct {
	Properties []PropertyInfo
	Enum       []string
	Type       string
}

type Registry struct {
	compiled  map[string]*jsonschema.Schema
	kindIndex map[string][]string
}

func NewRegistry() *Registry {
	return &Registry{
		compiled:  make(map[string]*jsonschema.Schema),
		kindIndex: make(map[string][]string),
	}
}

func (r *Registry) Load(schemas []ResourceSchema) error {
	type bundleEntry struct {
		bundleJSON []byte
		bundleURL  string
		schemas    []ResourceSchema
	}
	bundleMap := make(map[string]*bundleEntry)
	for _, s := range schemas {
		if _, ok := bundleMap[s.BundleURL]; !ok {
			bundleMap[s.BundleURL] = &bundleEntry{
				bundleJSON: s.BundleJSON,
				bundleURL:  s.BundleURL,
			}
		}
		bundleMap[s.BundleURL].schemas = append(bundleMap[s.BundleURL].schemas, s)
	}

	for _, entry := range bundleMap {
		c := jsonschema.NewCompiler()
		if err := c.AddResource(entry.bundleURL, strings.NewReader(string(entry.bundleJSON))); err != nil {
			return fmt.Errorf("add resource %s: %w", entry.bundleURL, err)
		}

		for _, s := range entry.schemas {
			compiled, err := c.Compile(s.SchemaRef)
			if err != nil {
				continue
			}

			av := apiVersion(s.GVK)
			key := s.GVK.Kind + "/" + av
			r.compiled[key] = compiled
			r.kindIndex[s.GVK.Kind] = appendUnique(r.kindIndex[s.GVK.Kind], av)
		}
	}
	return nil
}

func (r *Registry) Lookup(kind, av string) *jsonschema.Schema {
	return r.compiled[kind+"/"+av]
}

func (r *Registry) AllKinds() []string {
	kinds := make([]string, 0, len(r.kindIndex))
	for k := range r.kindIndex {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	return kinds
}

func (r *Registry) APIVersionsForKind(kind string) []string {
	avs := make([]string, len(r.kindIndex[kind]))
	copy(avs, r.kindIndex[kind])
	sort.Strings(avs)
	return avs
}

func (r *Registry) PropertiesAtPath(kind, av string, path []string) *PathInfo {
	s := r.Lookup(kind, av)
	if s == nil {
		return nil
	}
	return schemaAtPath(s, path)
}

func resolveSchema(s *jsonschema.Schema) *jsonschema.Schema {
	for {
		if s.Ref != nil {
			s = s.Ref
			continue
		}
		if len(s.AllOf) == 1 {
			s = s.AllOf[0]
			continue
		}
		return s
	}
}

func schemaAtPath(s *jsonschema.Schema, path []string) *PathInfo {
	s = resolveSchema(s)

	if len(path) == 0 {
		return schemaToPathInfo(s)
	}

	key := path[0]
	rest := path[1:]

	if s.Properties != nil {
		if child, ok := s.Properties[key]; ok {
			return schemaAtPath(child, rest)
		}
	}

	if s.Items != nil {
		if item, ok := s.Items.(*jsonschema.Schema); ok {
			return schemaAtPath(item, path)
		}
	}

	return nil
}

func schemaToPathInfo(s *jsonschema.Schema) *PathInfo {
	s = resolveSchema(s)
	info := &PathInfo{}

	if len(s.Types) > 0 {
		for _, t := range s.Types {
			if t != "null" {
				info.Type = t
				break
			}
		}
	}

	if len(s.Enum) > 0 {
		for _, v := range s.Enum {
			if sv, ok := v.(string); ok {
				info.Enum = append(info.Enum, sv)
			}
		}
	}

	if s.Properties != nil {
		requiredSet := make(map[string]bool, len(s.Required))
		for _, r := range s.Required {
			requiredSet[r] = true
		}

		for name, rawChild := range s.Properties {
			child := resolveSchema(rawChild)
			pi := PropertyInfo{
				Name:     name,
				Required: requiredSet[name],
			}
			if len(child.Types) > 0 {
				for _, t := range child.Types {
					if t != "null" {
						pi.Type = t
						break
					}
				}
			}
			if pi.Type == "array" {
				pi.IsArray = true
			}
			if len(child.Enum) > 0 {
				for _, v := range child.Enum {
					if sv, ok := v.(string); ok {
						pi.Enum = append(pi.Enum, sv)
					}
				}
			}
			info.Properties = append(info.Properties, pi)
		}
		sort.Slice(info.Properties, func(i, j int) bool {
			return info.Properties[i].Name < info.Properties[j].Name
		})
	}

	return info
}

func apiVersion(gvk GroupVersionKind) string {
	if gvk.Group == "" {
		return gvk.Version
	}
	return gvk.Group + "/" + gvk.Version
}

func appendUnique(slice []string, s string) []string {
	for _, v := range slice {
		if v == s {
			return slice
		}
	}
	return append(slice, s)
}
