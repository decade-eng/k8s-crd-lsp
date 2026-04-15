package schema

type GroupVersionKind struct {
	Group   string
	Version string
	Kind    string
}

type ResourceSchema struct {
	GVK        GroupVersionKind
	BundleJSON []byte
	BundleURL  string
	SchemaRef  string
}
