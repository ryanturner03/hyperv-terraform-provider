package client

import (
	"strings"
	"testing"
)

func TestBuildNewVMCommand(t *testing.T) {
	opts := VMOptions{
		Name:               "test-vm",
		Generation:         2,
		MemoryStartupBytes: 1073741824,
	}
	cmd := buildNewVMCommand(opts)
	expected := "New-VM -Name 'test-vm' -Generation 2 -MemoryStartupBytes 1073741824 -NoVHD -ErrorAction Stop"
	if cmd != expected {
		t.Errorf("got %q, want %q", cmd, expected)
	}
}

func TestBuildNewVMCommandWithInjection(t *testing.T) {
	opts := VMOptions{
		Name:               "test'; Remove-VM *; '",
		Generation:         2,
		MemoryStartupBytes: 1073741824,
	}
	cmd := buildNewVMCommand(opts)
	if cmd != "New-VM -Name 'test''; Remove-VM *; ''' -Generation 2 -MemoryStartupBytes 1073741824 -NoVHD -ErrorAction Stop" {
		t.Errorf("injection not escaped properly: %q", cmd)
	}
}

func TestBuildSetVMCommand(t *testing.T) {
	opts := VMOptions{
		Name:           "test-vm",
		ProcessorCount: 4,
		DynamicMemory:  true,
		Notes:          "my notes",
	}
	cmd := buildSetVMCommand(opts)
	// Should contain Set-VM with processor and dynamic memory params
	if cmd == "" {
		t.Error("expected non-empty command")
	}
}

func TestBuildSetVMFirmwareCommand(t *testing.T) {
	boolTrue := true
	boolFalse := false

	tests := []struct {
		name     string
		vmName   string
		opts     VMFirmwareOptions
		contains []string
	}{
		{
			name:   "enable secure boot",
			vmName: "test-vm",
			opts:   VMFirmwareOptions{SecureBootEnabled: &boolTrue},
			contains: []string{
				"Set-VMFirmware -VMName 'test-vm'",
				"-EnableSecureBoot On",
				"-ErrorAction Stop",
			},
		},
		{
			name:   "disable secure boot",
			vmName: "test-vm",
			opts:   VMFirmwareOptions{SecureBootEnabled: &boolFalse},
			contains: []string{
				"-EnableSecureBoot Off",
			},
		},
		{
			name:   "set template",
			vmName: "test-vm",
			opts:   VMFirmwareOptions{SecureBootTemplate: "MicrosoftUEFICertificateAuthority"},
			contains: []string{
				"-SecureBootTemplate 'MicrosoftUEFICertificateAuthority'",
			},
		},
		{
			name:   "combined secure boot and template",
			vmName: "my-vm",
			opts: VMFirmwareOptions{
				SecureBootEnabled:  &boolFalse,
				SecureBootTemplate: "MicrosoftWindows",
			},
			contains: []string{
				"-EnableSecureBoot Off",
				"-SecureBootTemplate 'MicrosoftWindows'",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildSetVMFirmwareCommand(tt.vmName, tt.opts)
			for _, s := range tt.contains {
				if !containsStr(cmd, s) {
					t.Errorf("command %q missing expected substring %q", cmd, s)
				}
			}
		})
	}
}

func TestBuildSetVMFirstBootDeviceCommand(t *testing.T) {
	tests := []struct {
		name     string
		vmName   string
		device   BootDevice
		contains []string
	}{
		{
			name:   "hard disk drive",
			vmName: "test-vm",
			device: BootDevice{DeviceType: "HardDiskDrive", ControllerNumber: 0, ControllerLocation: 0},
			contains: []string{
				"Get-VMHardDiskDrive -VMName 'test-vm'",
				"ControllerNumber -eq 0",
				"ControllerLocation -eq 0",
				"Set-VMFirmware -VMName 'test-vm' -FirstBootDevice $dev",
			},
		},
		{
			name:   "dvd drive",
			vmName: "test-vm",
			device: BootDevice{DeviceType: "DvdDrive", ControllerNumber: 0, ControllerLocation: 1},
			contains: []string{
				"Get-VMDvdDrive -VMName 'test-vm'",
				"ControllerLocation -eq 1",
			},
		},
		{
			name:   "network adapter",
			vmName: "test-vm",
			device: BootDevice{DeviceType: "NetworkAdapter"},
			contains: []string{
				"Get-VMNetworkAdapter -VMName 'test-vm'",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildSetVMFirstBootDeviceCommand(tt.vmName, tt.device)
			for _, s := range tt.contains {
				if !containsStr(cmd, s) {
					t.Errorf("command %q missing expected substring %q", cmd, s)
				}
			}
		})
	}
}

func TestBuildGetVMFirmwareCommand(t *testing.T) {
	cmd := buildGetVMFirmwareCommand("test-vm")
	expected := []string{
		"Get-VMFirmware -VMName 'test-vm'",
		"ConvertTo-Json",
		"SecureBootEnabled",
		"SecureBootTemplate",
		"FirstBootDeviceType",
	}
	for _, s := range expected {
		if !containsStr(cmd, s) {
			t.Errorf("command missing expected substring %q", s)
		}
	}
}

func TestBuildSetVMFirmwareCommandInjection(t *testing.T) {
	boolTrue := true
	cmd := buildSetVMFirmwareCommand("test'; Remove-VM *; '", VMFirmwareOptions{SecureBootEnabled: &boolTrue})
	// The name should be escaped with doubled single quotes: 'test''; Remove-VM *; '''
	// This means PowerShell treats it as the literal string, not as an injection.
	if !containsStr(cmd, "'test''") {
		t.Errorf("injection not properly escaped: %q", cmd)
	}
}

func containsStr(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
