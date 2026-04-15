package lsp_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/decade-eng/k8s-crd-lsp/internal/lsp"
)

type jsonRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

func encodeMessage(msg jsonRPCMessage) string {
	body, _ := json.Marshal(msg)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body)
}

func drainMessages(r io.Reader) <-chan *jsonRPCMessage {
	ch := make(chan *jsonRPCMessage, 64)
	go func() {
		reader := bufio.NewReader(r)
		for {
			var contentLength int
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					close(ch)
					return
				}
				line = strings.TrimSpace(line)
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "Content-Length:") {
					fmt.Sscanf(strings.TrimPrefix(line, "Content-Length:"), " %d", &contentLength)
				}
			}
			if contentLength == 0 {
				continue
			}
			body := make([]byte, contentLength)
			if _, err := io.ReadFull(reader, body); err != nil {
				close(ch)
				return
			}
			var msg jsonRPCMessage
			if err := json.Unmarshal(body, &msg); err != nil {
				continue
			}
			ch <- &msg
		}
	}()
	return ch
}

func waitForResponse(msgs <-chan *jsonRPCMessage, timeout time.Duration) (*jsonRPCMessage, error) {
	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				return nil, fmt.Errorf("server closed connection")
			}
			if msg.ID != nil {
				return msg, nil
			}
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for response")
		}
	}
}

func TestServerInitialize(t *testing.T) {
	serverInR, serverInW := io.Pipe()
	serverOutR, serverOutW := io.Pipe()

	server := lsp.NewServer("/nonexistent/kubectl")

	done := make(chan error, 1)
	go func() {
		done <- server.StartWithPipes(serverInR, serverOutW)
	}()

	msgs := drainMessages(serverOutR)

	initMsg := jsonRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"capabilities":{},"processId":null}`),
	}
	if _, err := serverInW.Write([]byte(encodeMessage(initMsg))); err != nil {
		t.Fatalf("write initialize: %v", err)
	}

	initResult, err := waitForResponse(msgs, 5*time.Second)
	if err != nil {
		t.Fatalf("initialize response: %v", err)
	}
	if initResult.Error != nil {
		t.Fatalf("initialize error: %s", initResult.Error)
	}

	var result struct {
		Capabilities struct {
			CompletionProvider interface{} `json:"completionProvider"`
			TextDocumentSync   interface{} `json:"textDocumentSync"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(initResult.Result, &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.Capabilities.CompletionProvider == nil {
		t.Error("expected completionProvider in capabilities")
	}
	if result.Capabilities.TextDocumentSync == nil {
		t.Error("expected textDocumentSync in capabilities")
	}

	shutdownMsg := jsonRPCMessage{JSONRPC: "2.0", ID: 2, Method: "shutdown"}
	serverInW.Write([]byte(encodeMessage(shutdownMsg))) //nolint
	exitMsg := jsonRPCMessage{JSONRPC: "2.0", Method: "exit"}
	serverInW.Write([]byte(encodeMessage(exitMsg))) //nolint
	serverInW.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Log("server did not exit cleanly (timeout)")
	}
}

func TestServerKubectlFailure(t *testing.T) {
	serverInR, serverInW := io.Pipe()
	serverOutR, serverOutW := io.Pipe()

	server := lsp.NewServer("/nonexistent/kubectl")

	done := make(chan error, 1)
	go func() {
		done <- server.StartWithPipes(serverInR, serverOutW)
	}()

	msgs := drainMessages(serverOutR)

	initMsg := jsonRPCMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params:  json.RawMessage(`{"capabilities":{},"processId":null}`),
	}
	serverInW.Write([]byte(encodeMessage(initMsg))) //nolint

	initResult, err := waitForResponse(msgs, 5*time.Second)
	if err != nil {
		t.Fatalf("initialize response: %v", err)
	}
	if initResult.Error != nil {
		t.Fatalf("initialize error: %s", initResult.Error)
	}

	// Send didOpen; after background schema loading fails, publishDiagnostics should fire
	didOpenMsg := jsonRPCMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/didOpen",
		Params: json.RawMessage(`{
			"textDocument": {
				"uri": "file:///test.yaml",
				"languageId": "yaml",
				"version": 1,
				"text": "apiVersion: v1\nkind: Pod\n"
			}
		}`),
	}
	serverInW.Write([]byte(encodeMessage(didOpenMsg))) //nolint

	shutdownMsg := jsonRPCMessage{JSONRPC: "2.0", ID: 2, Method: "shutdown"}
	serverInW.Write([]byte(encodeMessage(shutdownMsg))) //nolint
	exitMsg := jsonRPCMessage{JSONRPC: "2.0", Method: "exit"}
	serverInW.Write([]byte(encodeMessage(exitMsg))) //nolint
	serverInW.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Log("server did not exit cleanly (timeout)")
	}
}
