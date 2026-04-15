package yaml

import (
	"io"
	"strings"

	goyaml "gopkg.in/yaml.v3"
)

type Document struct {
	Root       *goyaml.Node
	LineOffset int
	Kind       string
	APIVersion string
}

func ParseFile(content string) []*Document {
	var docs []*Document
	dec := goyaml.NewDecoder(strings.NewReader(content))

	for {
		var node goyaml.Node
		err := dec.Decode(&node)
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		if node.Kind == 0 {
			break
		}

		doc := &Document{Root: &node}

		if node.Kind == goyaml.DocumentNode {
			doc.LineOffset = node.Line - 1 // 1-based to 0-based
		}

		if node.Kind == goyaml.DocumentNode && len(node.Content) > 0 {
			mapping := node.Content[0]
			if mapping.Kind == goyaml.MappingNode {
				doc.Kind, doc.APIVersion = extractKindAPIVersion(mapping)
			}
		}

		docs = append(docs, doc)
	}

	return docs
}

func DocumentAtPosition(docs []*Document, line int) *Document {
	var result *Document
	for _, d := range docs {
		if d.LineOffset <= line {
			result = d
		} else {
			break
		}
	}
	return result
}

func extractKindAPIVersion(mapping *goyaml.Node) (kind, apiVersion string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		val := mapping.Content[i+1]
		if key.Kind == goyaml.ScalarNode && val.Kind == goyaml.ScalarNode {
			switch key.Value {
			case "kind":
				kind = val.Value
			case "apiVersion":
				apiVersion = val.Value
			}
		}
	}
	return
}
