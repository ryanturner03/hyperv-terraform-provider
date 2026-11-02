package client

import (
	"context"
	"strings"
	"testing"
)

func TestBuildRemoveDefaultDVDCommand(t *testing.T) {
	cmd := buildRemoveDefaultDVDCommand("test'; Remove-VM *; '")
	expected := "Get-VMDvdDrive -VMName 'test''; Remove-VM *; ''' -ErrorAction Stop | Remove-VMDvdDrive -ErrorAction Stop"
	if cmd != expected {
		t.Errorf("got %q, want %q", cmd, expected)
	}
}

func createVMCommands(t *testing.T, generation int) []string {
	t.Helper()
	mock := &mockPS{jsonResponses: []any{VM{Name: "vm1", Generation: generation}}}
	c := newTestClient(mock)
	if _, err := c.CreateVM(context.Background(), VMOptions{Name: "vm1", Generation: generation, MemoryStartupBytes: 1 << 30}); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}
	return mock.commands
}

func hasCommand(cmds []string, substr string) bool {
	for _, c := range cmds {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

// Hyper-V auto-creates an empty DVD drive at IDE 1:0 on every Generation 1 VM.
// Terraform-declared dvd_drive blocks must be the only drives present, or the
// post-apply read-back reports one more block than configured.
func TestCreateVM_Gen1_RemovesDefaultDVDDrive(t *testing.T) {
	cmds := createVMCommands(t, 1)
	if !hasCommand(cmds, "Remove-VMDvdDrive") {
		t.Errorf("expected default DVD drive removal on Gen1, commands: %q", cmds)
	}
}

func TestCreateVM_Gen2_DoesNotTouchDVDDrives(t *testing.T) {
	cmds := createVMCommands(t, 2)
	if hasCommand(cmds, "Remove-VMDvdDrive") {
		t.Errorf("Gen2 create must not remove DVD drives, commands: %q", cmds)
	}
}
