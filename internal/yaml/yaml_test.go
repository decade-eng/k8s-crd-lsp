package yaml_test

import (
	"testing"

	myyaml "github.com/decade-eng/k8s-crd-lsp/internal/yaml"
)

const deploymentYAML = `apiVersion: apps/v1
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

const multiDocYAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy1
---
apiVersion: v1
kind: Service
metadata:
  name: svc1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
`

func TestParseFileSingleDoc(t *testing.T) {
	docs := myyaml.ParseFile(deploymentYAML)
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].Kind != "Deployment" {
		t.Errorf("expected Kind=Deployment, got %q", docs[0].Kind)
	}
	if docs[0].APIVersion != "apps/v1" {
		t.Errorf("expected APIVersion=apps/v1, got %q", docs[0].APIVersion)
	}
}

func TestParseFileMultiDoc(t *testing.T) {
	docs := myyaml.ParseFile(multiDocYAML)
	if len(docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(docs))
	}

	expected := []struct{ kind, apiVersion string }{
		{"Deployment", "apps/v1"},
		{"Service", "v1"},
		{"ConfigMap", "v1"},
	}
	for i, e := range expected {
		if docs[i].Kind != e.kind {
			t.Errorf("doc[%d]: expected kind %q, got %q", i, e.kind, docs[i].Kind)
		}
		if docs[i].APIVersion != e.apiVersion {
			t.Errorf("doc[%d]: expected apiVersion %q, got %q", i, e.apiVersion, docs[i].APIVersion)
		}
	}

	if docs[1].LineOffset <= docs[0].LineOffset {
		t.Errorf("doc[1].LineOffset=%d should be > doc[0].LineOffset=%d",
			docs[1].LineOffset, docs[0].LineOffset)
	}
	if docs[2].LineOffset <= docs[1].LineOffset {
		t.Errorf("doc[2].LineOffset=%d should be > doc[1].LineOffset=%d",
			docs[2].LineOffset, docs[1].LineOffset)
	}
}

func TestDocumentAtPosition(t *testing.T) {
	docs := myyaml.ParseFile(multiDocYAML)
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}

	d := myyaml.DocumentAtPosition(docs, 0)
	if d != docs[0] {
		t.Error("line 0 should be in doc[0]")
	}

	d = myyaml.DocumentAtPosition(docs, docs[1].LineOffset)
	if d != docs[1] {
		t.Errorf("line %d should be in doc[1]", docs[1].LineOffset)
	}

	d = myyaml.DocumentAtPosition(docs, docs[2].LineOffset+1)
	if d != docs[2] {
		t.Errorf("line %d should be in doc[2]", docs[2].LineOffset+1)
	}
}

func TestDocumentAtPositionNil(t *testing.T) {
	d := myyaml.DocumentAtPosition(nil, 0)
	if d != nil {
		t.Error("expected nil for empty docs")
	}
}

func TestMultiDocParsing(t *testing.T) {
	docs := myyaml.ParseFile(multiDocYAML)
	if len(docs) != 3 {
		t.Fatalf("expected 3 documents, got %d", len(docs))
	}
	t.Logf("doc[0]: kind=%s offset=%d", docs[0].Kind, docs[0].LineOffset)
	t.Logf("doc[1]: kind=%s offset=%d", docs[1].Kind, docs[1].LineOffset)
	t.Logf("doc[2]: kind=%s offset=%d", docs[2].Kind, docs[2].LineOffset)
}

func TestParseFileEmpty(t *testing.T) {
	docs := myyaml.ParseFile("")
	if len(docs) != 0 {
		t.Errorf("expected 0 documents for empty input, got %d", len(docs))
	}
}

func TestParseFileInvalid(t *testing.T) {
	docs := myyaml.ParseFile(":\n  :\n  :\n  [invalid")
	_ = docs
}

func TestLastKnownGood(t *testing.T) {
	store := myyaml.NewStore()

	store.Update("file:///test.yaml", deploymentYAML)
	docs := store.Get("file:///test.yaml")
	if len(docs) == 0 {
		t.Fatal("expected docs after valid update")
	}
	prevDocs := docs

	store.Update("file:///test.yaml", "")
	docs = store.Get("file:///test.yaml")
	if len(docs) == 0 {
		t.Error("expected to retain previous docs after empty update")
	}
	if docs[0] != prevDocs[0] {
		t.Error("expected same docs retained after failed update")
	}
}

func TestStoreRemove(t *testing.T) {
	store := myyaml.NewStore()
	store.Update("file:///a.yaml", deploymentYAML)
	if store.Get("file:///a.yaml") == nil {
		t.Fatal("expected docs")
	}
	store.Remove("file:///a.yaml")
	if store.Get("file:///a.yaml") != nil {
		t.Error("expected nil after remove")
	}
}

func TestStoreGetMissing(t *testing.T) {
	store := myyaml.NewStore()
	if store.Get("file:///nonexistent.yaml") != nil {
		t.Error("expected nil for missing URI")
	}
}

func TestPositionMapping(t *testing.T) {
	docs := myyaml.ParseFile(deploymentYAML)
	if len(docs) == 0 {
		t.Fatal("no docs parsed")
	}
	doc := docs[0]

	ctx := myyaml.CompletionContext(doc, 0, 0)
	t.Logf("line 0 ctx: type=%v path=%v", ctx.Type, ctx.Path)

	ctx = myyaml.CompletionContext(doc, 5, 0)
	t.Logf("line 5 (spec:) ctx: type=%v path=%v", ctx.Type, ctx.Path)

	ctx = myyaml.CompletionContext(doc, 6, 2)
	t.Logf("line 6 (replicas) ctx: type=%v path=%v", ctx.Type, ctx.Path)
}

func TestCompletionContextNilDoc(t *testing.T) {
	ctx := myyaml.CompletionContext(nil, 0, 0)
	if ctx.Type != myyaml.CtxPropertyName {
		t.Errorf("expected CtxPropertyName for nil doc, got %v", ctx.Type)
	}
}

func TestNodeAtPositionReturnsNonNil(t *testing.T) {
	docs := myyaml.ParseFile(deploymentYAML)
	if len(docs) == 0 {
		t.Fatal("no docs parsed")
	}

	root := docs[0].Root
	node := myyaml.NodeAtPosition(root, 0, 0, 0)
	if node == nil {
		t.Error("expected non-nil node at (0,0)")
	}
}

func TestPathToNodeRoot(t *testing.T) {
	docs := myyaml.ParseFile(deploymentYAML)
	if len(docs) == 0 {
		t.Fatal("no docs")
	}
	path := myyaml.PathToNode(docs[0].Root, docs[0].Root)
	if len(path) != 0 {
		t.Errorf("expected empty path for root, got %v", path)
	}
}

func TestPathToNodeNil(t *testing.T) {
	path := myyaml.PathToNode(nil, nil)
	if path != nil {
		t.Errorf("expected nil for nil inputs, got %v", path)
	}
}
