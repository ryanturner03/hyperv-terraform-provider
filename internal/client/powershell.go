package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/masterzen/winrm"
)

// EscapePSString wraps a value in single quotes with internal single quotes doubled.
func EscapePSString(s string) string {
	escaped := strings.ReplaceAll(s, "'", "''")
	return "'" + escaped + "'"
}

// PowerShellRunner executes PowerShell commands over a WinRM connection.
type PowerShellRunner struct {
	client *winrm.Client
}

// NewPowerShellRunner creates a new PowerShellRunner wrapping the given WinRM client.
func NewPowerShellRunner(client *winrm.Client) *PowerShellRunner {
	return &PowerShellRunner{client: client}
}

// Run executes a PowerShell command and returns stdout, stderr, and any error.
func (r *PowerShellRunner) Run(ctx context.Context, command string) (string, string, error) {
	stdout, stderr, exitCode, err := r.client.RunPSWithContextWithString(ctx, command, "")
	if err != nil {
		return "", "", fmt.Errorf("winrm error: %w", err)
	}
	if exitCode != 0 {
		return stdout, stderr, fmt.Errorf("powershell exited with code %d: %s", exitCode, strings.TrimSpace(stderr))
	}
	return strings.TrimSpace(stdout), strings.TrimSpace(stderr), nil
}

// RunJSON executes a PowerShell command and unmarshals the JSON output into result.
// Handles PowerShell's ConvertTo-Json which may return an array even for single objects.
func (r *PowerShellRunner) RunJSON(ctx context.Context, command string, result any) error {
	stdout, stderr, err := r.Run(ctx, command)
	if err != nil {
		if stderr != "" {
			return fmt.Errorf("%w (stderr: %s)", err, stderr)
		}
		return err
	}
	if stdout == "" {
		return fmt.Errorf("empty output from PowerShell command")
	}
	if err := json.Unmarshal([]byte(stdout), result); err != nil {
		// PowerShell may wrap single objects in an array — try unwrapping
		trimmed := strings.TrimSpace(stdout)
		if len(trimmed) > 0 && trimmed[0] == '[' {
			var raw []json.RawMessage
			if err2 := json.Unmarshal([]byte(trimmed), &raw); err2 == nil && len(raw) > 0 {
				if len(raw) > 1 {
					return fmt.Errorf("expected single JSON object but got array of %d elements (output was: %.200s)", len(raw), stdout)
				}
				if err3 := json.Unmarshal(raw[0], result); err3 == nil {
					return nil
				}
			}
		}
		return fmt.Errorf("failed to unmarshal JSON: %w (output was: %.200s)", err, stdout)
	}
	return nil
}
