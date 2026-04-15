package kubectl

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

const defaultTimeout = 30 * time.Second

type Executor struct {
	kubectlPath string
}

func New(kubectlPath string) *Executor {
	return &Executor{kubectlPath: kubectlPath}
}

func (e *Executor) Run(args ...string) ([]byte, error) {
	path := e.kubectlPath
	if path == "" {
		var err error
		path, err = exec.LookPath("kubectl")
		if err != nil {
			return nil, fmt.Errorf("kubectl not found on PATH: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("kubectl %v: %s", args, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("kubectl %v: %w", args, err)
	}
	return out, nil
}
