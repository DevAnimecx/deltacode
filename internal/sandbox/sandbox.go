package sandbox

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Sandbox struct {
	Dir string
}

func New() (*Sandbox, error) {
	dir, err := os.MkdirTemp("", "delta-sandbox-*")
	if err != nil {
		return nil, fmt.Errorf("cannot create sandbox: %w", err)
	}
	return &Sandbox{Dir: dir}, nil
}

func (s *Sandbox) WriteFile(path, content string) error {
	fullPath := filepath.Join(s.Dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(content), 0644)
}

func (s *Sandbox) RunCommand(command string, args ...string) (*Result, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = s.Dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, err
		}
	}

	return &Result{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: exitCode,
	}, nil
}

func (s *Sandbox) RunShell(command string) (*Result, error) {
	var shell, flag string
	if _, err := exec.LookPath("powershell"); err == nil {
		shell = "powershell"
		flag = "-Command"
	} else {
		shell = "cmd"
		flag = "/C"
	}
	return s.RunCommand(shell, flag, command)
}

func (s *Sandbox) Cleanup() error {
	return os.RemoveAll(s.Dir)
}
