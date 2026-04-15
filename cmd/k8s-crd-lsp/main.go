package main

import (
	"flag"
	"os"

	"github.com/decade-eng/k8s-crd-lsp/internal/lsp"
)

func main() {
	kubectlPath := flag.String("kubectl-path", "", "Path to kubectl binary (default: find on PATH)")
	flag.Parse()

	server := lsp.NewServer(*kubectlPath)
	if err := server.Start(); err != nil {
		os.Exit(1)
	}
}
