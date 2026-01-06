package exec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// PythonExecutor executes Python code via subprocess.
type PythonExecutor struct {
	pythonPath string
	timeout    time.Duration
	imports    []string
}

// PythonOption configures PythonExecutor.
type PythonOption func(*PythonExecutor)

// WithPythonPath sets the Python interpreter path.
func WithPythonPath(path string) PythonOption {
	return func(e *PythonExecutor) { e.pythonPath = path }
}

// WithTimeout sets execution timeout.
func WithTimeout(d time.Duration) PythonOption {
	return func(e *PythonExecutor) { e.timeout = d }
}

// WithImports sets allowed imports.
func WithImports(imports []string) PythonOption {
	return func(e *PythonExecutor) { e.imports = imports }
}

// NewPythonExecutor creates a Python code executor.
func NewPythonExecutor(opts ...PythonOption) *PythonExecutor {
	e := &PythonExecutor{
		pythonPath: "python3",
		timeout:    30 * time.Second,
		imports:    []string{"math", "json", "datetime", "re", "collections"},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs Python code and returns output.
func (e *PythonExecutor) Execute(code string, state map[string]any) (any, string, error) {
	// Wrap code with state injection and output capture
	wrappedCode := e.wrapCode(code, state)

	cmd := exec.Command(e.pythonPath, "-c", wrappedCode)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		logs := stdout.String()
		if err != nil {
			return nil, logs, fmt.Errorf("%v: %s", err, stderr.String())
		}
		// Parse result from stdout (last line is JSON result)
		lines := strings.Split(strings.TrimSpace(logs), "\n")
		if len(lines) == 0 {
			return nil, logs, nil
		}

		// Check for final answer marker
		lastLine := lines[len(lines)-1]
		if strings.HasPrefix(lastLine, "__RESULT__:") {
			resultJSON := strings.TrimPrefix(lastLine, "__RESULT__:")
			var result any
			json.Unmarshal([]byte(resultJSON), &result)
			return result, strings.Join(lines[:len(lines)-1], "\n"), nil
		}
		return nil, logs, nil

	case <-time.After(e.timeout):
		cmd.Process.Kill()
		return nil, "", fmt.Errorf("execution timeout after %v", e.timeout)
	}
}

func (e *PythonExecutor) wrapCode(code string, state map[string]any) string {
	stateJSON, _ := json.Marshal(state)

	return fmt.Sprintf(`
import json
import sys
import math

# Inject state
__state__ = json.loads('%s')
locals().update(__state__)

# Capture final answer
__final_answer__ = None
def final_answer(answer):
    global __final_answer__
    __final_answer__ = answer
    print(f"Final Answer: {answer}")
    return answer

# Execute user code
%s

# Output result
if __final_answer__ is not None:
    try:
        print("__RESULT__:" + json.dumps(__final_answer__))
    except (TypeError, ValueError):
        print("__RESULT__:" + json.dumps(str(__final_answer__)))
`, string(stateJSON), code)
}

// DockerExecutor executes code in a Docker container.
type DockerExecutor struct {
	image   string
	timeout time.Duration
}

// NewDockerExecutor creates a Docker-based executor.
func NewDockerExecutor(image string, timeout time.Duration) *DockerExecutor {
	if image == "" {
		image = "python:3.11-slim"
	}
	return &DockerExecutor{image: image, timeout: timeout}
}

// Execute runs code in Docker container.
func (e *DockerExecutor) Execute(code string, state map[string]any) (any, string, error) {
	stateJSON, _ := json.Marshal(state)

	wrappedCode := fmt.Sprintf(`
import json
__state__ = json.loads('%s')
locals().update(__state__)
__final_answer__ = None
def final_answer(answer):
    global __final_answer__
    __final_answer__ = answer
    return answer
%s
if __final_answer__ is not None:
    print("__RESULT__:" + json.dumps(__final_answer__))
`, string(stateJSON), code)

	cmd := exec.Command("docker", "run", "--rm", "-i",
		"--network=none",
		"--memory=256m",
		"--cpus=0.5",
		e.image,
		"python3", "-c", wrappedCode)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		logs := stdout.String()
		if err != nil {
			return nil, logs, fmt.Errorf("%v: %s", err, stderr.String())
		}
		lines := strings.Split(strings.TrimSpace(logs), "\n")
		if len(lines) > 0 {
			lastLine := lines[len(lines)-1]
			if after, ok := strings.CutPrefix(lastLine, "__RESULT__:"); ok {
				resultJSON := after
				var result any
				json.Unmarshal([]byte(resultJSON), &result)
				return result, strings.Join(lines[:len(lines)-1], "\n"), nil
			}
		}
		return nil, logs, nil
	case <-time.After(e.timeout):
		return nil, "", fmt.Errorf("docker execution timeout")
	}
}
