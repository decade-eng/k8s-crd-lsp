package lsp

import (
	"github.com/decade-eng/k8s-crd-lsp/internal/schema"
	myyaml "github.com/decade-eng/k8s-crd-lsp/internal/yaml"
)

// CompletionItem is a simplified LSP completion item (will be converted to full LSP type by server).
type CompletionItem struct {
	Label      string
	Kind       int // LSP CompletionItemKind: 10=Property, 12=Value, 13=Enum
	Detail     string
	InsertText string
	SortText   string
}

func Provide(doc *myyaml.Document, line, col int, reg *schema.Registry) []CompletionItem {
	if doc == nil || reg == nil {
		return rootCompletions()
	}

	ctx := myyaml.CompletionContext(doc, line, col)

	switch ctx.Type {
	case myyaml.CtxPropertyName:
		return propertyNameCompletions(doc, ctx.Path, reg)
	case myyaml.CtxPropertyValue:
		return propertyValueCompletions(doc, ctx.Path, reg)
	case myyaml.CtxArrayItem:
		return propertyNameCompletions(doc, append(ctx.Path, "[]"), reg)
	default:
		return rootCompletions()
	}
}

var rootK8sFields = []string{"apiVersion", "kind", "metadata", "spec", "status"}

func rootCompletions() []CompletionItem {
	items := make([]CompletionItem, len(rootK8sFields))
	for i, f := range rootK8sFields {
		items[i] = CompletionItem{
			Label:      f,
			Kind:       10,
			InsertText: f + ": ",
			SortText:   f,
		}
	}
	return items
}

func propertyNameCompletions(doc *myyaml.Document, path []string, reg *schema.Registry) []CompletionItem {
	kind, av := doc.Kind, doc.APIVersion

	if kind == "" {
		return kindCompletions(reg)
	}
	if av == "" {
		return apiVersionCompletions(kind, reg)
	}

	if len(path) == 0 {
		return rootCompletions()
	}

	info := reg.PropertiesAtPath(kind, av, path)
	if info == nil {
		return nil
	}

	items := make([]CompletionItem, 0, len(info.Properties))
	for _, p := range info.Properties {
		item := CompletionItem{
			Label:      p.Name,
			Kind:       10,
			Detail:     p.Type,
			InsertText: p.Name + ": ",
			SortText:   p.Name,
		}
		if p.IsArray {
			item.InsertText = p.Name + ":\n  - "
		}
		items = append(items, item)
	}
	return items
}

func kindCompletions(reg *schema.Registry) []CompletionItem {
	kinds := reg.AllKinds()
	items := make([]CompletionItem, 0, len(kinds)+len(rootK8sFields))

	for _, f := range rootK8sFields {
		items = append(items, CompletionItem{
			Label:      f,
			Kind:       10,
			InsertText: f + ": ",
			SortText:   f,
		})
	}

	kindSet := make(map[string]bool)
	for _, item := range items {
		kindSet[item.Label] = true
	}
	for _, k := range kinds {
		if !kindSet[k] {
			items = append(items, CompletionItem{
				Label:      k,
				Kind:       13,
				Detail:     "kind",
				InsertText: k,
				SortText:   k,
			})
		}
	}
	return items
}

func apiVersionCompletions(kind string, reg *schema.Registry) []CompletionItem {
	avs := reg.APIVersionsForKind(kind)
	items := make([]CompletionItem, len(avs))
	for i, av := range avs {
		items[i] = CompletionItem{
			Label:      av,
			Kind:       13,
			Detail:     "apiVersion",
			InsertText: av,
			SortText:   av,
		}
	}
	return items
}

func propertyValueCompletions(doc *myyaml.Document, path []string, reg *schema.Registry) []CompletionItem {
	kind, av := doc.Kind, doc.APIVersion
	if kind == "" || av == "" {
		return nil
	}

	if len(path) == 0 {
		return nil
	}

	parentPath := path[:len(path)-1]
	propName := path[len(path)-1]

	if propName == "kind" && len(parentPath) == 0 {
		kinds := reg.AllKinds()
		items := make([]CompletionItem, len(kinds))
		for i, k := range kinds {
			items[i] = CompletionItem{Label: k, Kind: 13, InsertText: k, SortText: k}
		}
		return items
	}

	if propName == "apiVersion" && len(parentPath) == 0 {
		return apiVersionCompletions(kind, reg)
	}

	info := reg.PropertiesAtPath(kind, av, parentPath)
	if info == nil {
		return nil
	}

	for _, p := range info.Properties {
		if p.Name == propName && len(p.Enum) > 0 {
			items := make([]CompletionItem, len(p.Enum))
			for i, e := range p.Enum {
				items[i] = CompletionItem{
					Label:      e,
					Kind:       13,
					InsertText: e,
					SortText:   e,
				}
			}
			return items
		}
		if p.Name == propName && p.Type == "boolean" {
			return []CompletionItem{
				{Label: "true", Kind: 12, InsertText: "true"},
				{Label: "false", Kind: 12, InsertText: "false"},
			}
		}
	}

	return nil
}
