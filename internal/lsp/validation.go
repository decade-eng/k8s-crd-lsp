package lsp

import (
	"encoding/json"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	goyaml "gopkg.in/yaml.v3"

	"github.com/decade-eng/k8s-crd-lsp/internal/schema"
	myyaml "github.com/decade-eng/k8s-crd-lsp/internal/yaml"
)

const (
	SeverityError   = 1
	SeverityWarning = 2
	SeverityInfo    = 3
)

type Diagnostic struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Severity  int
	Message   string
	Source    string
}

func ValidateAll(docs []*myyaml.Document, reg *schema.Registry) []Diagnostic {
	var all []Diagnostic
	for _, doc := range docs {
		all = append(all, ValidateDoc(doc, reg)...)
	}
	return all
}

func ValidateDoc(doc *myyaml.Document, reg *schema.Registry) []Diagnostic {
	if doc == nil || doc.Root == nil {
		return nil
	}

	kind := doc.Kind
	av := doc.APIVersion

	if kind == "" && av == "" {
		return nil
	}

	if kind == "" {
		return []Diagnostic{{
			StartLine: doc.LineOffset, StartCol: 0,
			EndLine: doc.LineOffset, EndCol: 0,
			Severity: SeverityWarning,
			Message:  "missing 'kind' field",
			Source:   "k8s-crd-lsp",
		}}
	}
	if av == "" {
		return []Diagnostic{{
			StartLine: doc.LineOffset, StartCol: 0,
			EndLine: doc.LineOffset, EndCol: 0,
			Severity: SeverityWarning,
			Message:  "missing 'apiVersion' field",
			Source:   "k8s-crd-lsp",
		}}
	}

	s := reg.Lookup(kind, av)
	if s == nil {
		return []Diagnostic{{
			StartLine: doc.LineOffset, StartCol: 0,
			EndLine: doc.LineOffset, EndCol: 0,
			Severity: SeverityInfo,
			Message:  "unknown resource type: " + kind + "/" + av,
			Source:   "k8s-crd-lsp",
		}}
	}

	jsonVal, err := yamlNodeToJSONVal(doc.Root)
	if err != nil {
		return nil
	}

	validationErr := s.Validate(jsonVal)
	if validationErr == nil {
		return nil
	}

	ve, ok := validationErr.(*jsonschema.ValidationError)
	if !ok {
		return []Diagnostic{{
			StartLine: doc.LineOffset, StartCol: 0,
			EndLine: doc.LineOffset, EndCol: 0,
			Severity: SeverityError,
			Message:  validationErr.Error(),
			Source:   "k8s-crd-lsp",
		}}
	}

	return collectDiagnostics(ve, doc)
}

func collectDiagnostics(ve *jsonschema.ValidationError, doc *myyaml.Document) []Diagnostic {
	var diags []Diagnostic
	if len(ve.Causes) == 0 {
		diags = append(diags, errorToDiagnostic(ve, doc))
	} else {
		for _, cause := range ve.Causes {
			diags = append(diags, collectDiagnostics(cause, doc)...)
		}
	}
	return diags
}

func errorToDiagnostic(ve *jsonschema.ValidationError, doc *myyaml.Document) Diagnostic {
	path := jsonPointerToPath(ve.InstanceLocation)
	line, col := findNodePosition(doc.Root, path, doc.LineOffset)

	return Diagnostic{
		StartLine: line,
		StartCol:  col,
		EndLine:   line,
		EndCol:    col + 1,
		Severity:  SeverityError,
		Message:   ve.Message,
		Source:    "k8s-crd-lsp",
	}
}

func jsonPointerToPath(pointer string) []string {
	if pointer == "" || pointer == "/" {
		return nil
	}
	pointer = strings.TrimPrefix(pointer, "/")
	return strings.Split(pointer, "/")
}

func findNodePosition(root *goyaml.Node, path []string, lineOffset int) (line, col int) {
	if root == nil {
		return lineOffset, 0
	}
	node := navigateToPath(root, path)
	if node == nil {
		return lineOffset, 0
	}
	return node.Line - 1, node.Column - 1
}

func navigateToPath(node *goyaml.Node, path []string) *goyaml.Node {
	if len(path) == 0 {
		return node
	}
	if node.Kind == goyaml.DocumentNode && len(node.Content) > 0 {
		return navigateToPath(node.Content[0], path)
	}
	if node.Kind == goyaml.MappingNode {
		key := path[0]
		rest := path[1:]
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return navigateToPath(node.Content[i+1], rest)
			}
		}
		return nil
	}
	if node.Kind == goyaml.SequenceNode {
		idx := 0
		for _, c := range path[0] {
			if c < '0' || c > '9' {
				return nil
			}
			idx = idx*10 + int(c-'0')
		}
		if idx < len(node.Content) {
			return navigateToPath(node.Content[idx], path[1:])
		}
		return nil
	}
	return node
}

func yamlNodeToJSONVal(node *goyaml.Node) (any, error) {
	var obj any
	if err := node.Decode(&obj); err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var jsonVal any
	if err := json.Unmarshal(jsonBytes, &jsonVal); err != nil {
		return nil, err
	}
	return jsonVal, nil
}
