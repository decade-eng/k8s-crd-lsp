package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type KubectlRunner interface {
	Run(args ...string) ([]byte, error)
}

type DiscoveryResult struct {
	Paths map[string]string
}

func FetchContext(kubectl KubectlRunner) (string, error) {
	out, err := kubectl.Run("config", "current-context")
	if err != nil {
		return "", fmt.Errorf("get current context: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func FetchDiscovery(kubectl KubectlRunner) (*DiscoveryResult, error) {
	out, err := kubectl.Run("get", "--raw", "/openapi/v3")
	if err != nil {
		return nil, fmt.Errorf("fetch discovery: %w", err)
	}

	var doc struct {
		Paths map[string]struct {
			ServerRelativeURL string `json:"serverRelativeURL"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("parse discovery: %w", err)
	}

	result := &DiscoveryResult{Paths: make(map[string]string, len(doc.Paths))}
	for path, api := range doc.Paths {
		result.Paths[path] = api.ServerRelativeURL
	}
	return result, nil
}

func FetchAPIGroupSchema(kubectl KubectlRunner, serverRelativeURL string) ([]byte, error) {
	out, err := kubectl.Run("get", "--raw", serverRelativeURL)
	if err != nil {
		return nil, fmt.Errorf("fetch schema %s: %w", serverRelativeURL, err)
	}
	return out, nil
}

func FetchAllSchemas(kubectl KubectlRunner) ([]ResourceSchema, error) {
	discovery, err := FetchDiscovery(kubectl)
	if err != nil {
		return nil, err
	}

	type pathEntry struct {
		path string
		url  string
	}

	var entries []pathEntry
	for path, url := range discovery.Paths {
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if !((len(parts) == 2 && parts[0] == "api") || (len(parts) == 3 && parts[0] == "apis")) {
			continue
		}
		entries = append(entries, pathEntry{path, url})
	}

	const maxConcurrency = 5
	sem := make(chan struct{}, maxConcurrency)

	var mu sync.Mutex
	var results []ResourceSchema
	var wg sync.WaitGroup

	for _, entry := range entries {
		entry := entry
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			raw, err := FetchAPIGroupSchema(kubectl, entry.url)
			if err != nil {
				return
			}

			schemas, err := ParseAPIGroupSchemas(raw, entry.path)
			if err != nil {
				return
			}

			mu.Lock()
			results = append(results, schemas...)
			mu.Unlock()
		}()
	}

	wg.Wait()
	return results, nil
}
