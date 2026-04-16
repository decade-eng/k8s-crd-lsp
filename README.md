# k8s-crd-lsp

LSP server that provides schema-aware completions and validation for Kubernetes YAML files, including CRDs installed in your cluster.

## What it does

- Completions for `kind`, `apiVersion`, field names, and enum values — driven by your cluster's actual schemas
- Inline diagnostics for invalid field names, wrong types, and missing required fields
- Supports all built-in K8s resources and any CRDs installed in the cluster
- Works with any LSP-compatible editor

## How it works

On startup, the server calls `kubectl` to fetch the API discovery document, then lazily loads OpenAPI v3 schemas for each API group as you open files. Schemas are converted to JSON Schema and used for both completions and validation via `jsonschema`.

```
kubectl get --raw /openapi/v3  →  per-group OpenAPI v3  →  JSON Schema  →  completions + diagnostics
```

## Installation

**From releases** (macOS/Linux, amd64/arm64):

Download the binary from [GitHub Releases](https://github.com/decade-eng/k8s-crd-lsp/releases) and put it on your `PATH`.

**Via `go install`:**

```bash
go install github.com/decade-eng/k8s-crd-lsp/cmd/k8s-crd-lsp@latest
```

**Build from source:**

```bash
git clone https://github.com/decade-eng/k8s-crd-lsp.git
cd k8s-crd-lsp
go build -o k8s-crd-lsp ./cmd/k8s-crd-lsp
```

## Usage

The server communicates over stdin/stdout using LSP. Start it from your editor's LSP config:

```
k8s-crd-lsp
```

**Flag:**

```
--kubectl-path <path>   Path to kubectl binary (default: find on PATH)
```

### Zed

Install the [k8s-crd-lsp Zed extension](https://github.com/decade-eng/k8s-crd-lsp-zed). It downloads the binary automatically.

### Other editors

Configure any LSP client to run `k8s-crd-lsp` for YAML files. Example for Neovim with `nvim-lspconfig`:

```lua
vim.lsp.start({
  name = "k8s-crd-lsp",
  cmd = { "k8s-crd-lsp" },
  filetypes = { "yaml" },
  root_dir = vim.fn.getcwd(),
})
```

## Requirements

- `kubectl` on `PATH` (or passed via `--kubectl-path`)
- A valid `kubeconfig` pointing at a reachable cluster
- Kubernetes 1.27+ (OpenAPI v3 endpoint required)
