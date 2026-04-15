package yaml

import (
	goyaml "gopkg.in/yaml.v3"
)

type CtxType int

const (
	CtxUnknown CtxType = iota
	CtxPropertyName
	CtxPropertyValue
	CtxArrayItem
)

type CompletionCtx struct {
	Type CtxType
	Path []string
}

func NodeAtPosition(root *goyaml.Node, line, col, lineOffset int) *goyaml.Node {
	targetLine := line + 1 // 0-based to 1-based (yaml.v3 uses 1-based)
	targetCol := col + 1
	return findDeepest(root, targetLine, targetCol)
}

func findDeepest(node *goyaml.Node, line, col int) *goyaml.Node {
	if node == nil {
		return nil
	}

	for _, child := range node.Content {
		if result := findDeepest(child, line, col); result != nil {
			return result
		}
	}

	if nodeContains(node, line, col) {
		return node
	}

	return nil
}

func nodeContains(node *goyaml.Node, line, col int) bool {
	if node.Kind == goyaml.DocumentNode {
		return true
	}
	return node.Line == line
}

func PathToNode(root *goyaml.Node, target *goyaml.Node) []string {
	if root == nil || target == nil {
		return nil
	}
	var path []string
	if findPath(root, target, &path) {
		return path
	}
	return nil
}

func findPath(current, target *goyaml.Node, path *[]string) bool {
	if current == target {
		return true
	}
	if current.Kind == goyaml.MappingNode {
		for i := 0; i+1 < len(current.Content); i += 2 {
			key := current.Content[i]
			val := current.Content[i+1]
			*path = append(*path, key.Value)
			if findPath(val, target, path) {
				return true
			}
			*path = (*path)[:len(*path)-1]
		}
	} else {
		for _, child := range current.Content {
			if findPath(child, target, path) {
				return true
			}
		}
	}
	return false
}

func CompletionContext(doc *Document, line, col int) CompletionCtx {
	if doc == nil || doc.Root == nil {
		return CompletionCtx{Type: CtxPropertyName}
	}

	node := NodeAtPosition(doc.Root, line, col, doc.LineOffset)
	if node == nil {
		return CompletionCtx{Type: CtxPropertyName}
	}

	path := PathToNode(doc.Root, node)

	switch node.Kind {
	case goyaml.ScalarNode:
		if isValueNode(doc.Root, node) {
			return CompletionCtx{Type: CtxPropertyValue, Path: path}
		}
		if len(path) > 0 {
			return CompletionCtx{Type: CtxPropertyName, Path: path[:len(path)-1]}
		}
		return CompletionCtx{Type: CtxPropertyName}

	case goyaml.MappingNode, goyaml.DocumentNode:
		return CompletionCtx{Type: CtxPropertyName, Path: path}

	case goyaml.SequenceNode:
		return CompletionCtx{Type: CtxArrayItem, Path: path}
	}

	return CompletionCtx{Type: CtxPropertyName, Path: path}
}

func isValueNode(root, target *goyaml.Node) bool {
	return checkIsValue(root, target)
}

func checkIsValue(node, target *goyaml.Node) bool {
	if node.Kind == goyaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i+1] == target {
				return true
			}
		}
	}
	for _, child := range node.Content {
		if checkIsValue(child, target) {
			return true
		}
	}
	return false
}
