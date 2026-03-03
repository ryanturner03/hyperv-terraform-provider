package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (c *WinRMClient) CreateHardDrive(ctx context.Context, opts HardDriveOptions) (*HardDrive, error) {
	unlock := c.vmLock(opts.VMName)
	defer unlock()

	controllerType := opts.ControllerType
	if controllerType == "" {
		controllerType = "SCSI"
	}

	cmd := fmt.Sprintf("Add-VMHardDiskDrive -VMName %s -ControllerType %s -ControllerNumber %d",
		EscapePSString(opts.VMName), EscapePSString(controllerType), opts.ControllerNumber)
	if opts.ControllerLocationSet {
		cmd += fmt.Sprintf(" -ControllerLocation %d", opts.ControllerLocation)
	}
	if opts.Path != "" {
		cmd += fmt.Sprintf(" -Path %s", EscapePSString(opts.Path))
	}
	cmd += " -Passthru -ErrorAction Stop | Select-Object VMName,Path,ControllerType,ControllerNumber,ControllerLocation | ConvertTo-Json"

	var drive HardDrive
	err := retryOnConflict(ctx, 3, 3*time.Second, func() error {
		return c.ps.RunJSON(ctx, cmd, &drive)
	})
	if err != nil {
		return nil, fmt.Errorf("create hard drive on VM %q: %w", opts.VMName, err)
	}
	return &drive, nil
}

func (c *WinRMClient) GetHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int) (*HardDrive, error) {
	cmd := fmt.Sprintf(
		"Get-VMHardDiskDrive -VMName %s -ControllerType %s -ControllerNumber %d -ControllerLocation %d -ErrorAction Stop | Select-Object VMName,Path,ControllerType,ControllerNumber,ControllerLocation | ConvertTo-Json",
		EscapePSString(vmName), EscapePSString(controllerType), controllerNumber, controllerLocation,
	)
	var drive HardDrive
	err := c.ps.RunJSON(ctx, cmd, &drive)
	if err != nil {
		return nil, fmt.Errorf("get hard drive on VM %q (%s %d:%d): %w", vmName, controllerType, controllerNumber, controllerLocation, err)
	}
	return &drive, nil
}

func (c *WinRMClient) UpdateHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int, path string) error {
	unlock := c.vmLock(vmName)
	defer unlock()

	cmd := fmt.Sprintf(
		"Set-VMHardDiskDrive -VMName %s -ControllerType %s -ControllerNumber %d -ControllerLocation %d",
		EscapePSString(vmName), EscapePSString(controllerType), controllerNumber, controllerLocation,
	)
	if path != "" {
		cmd += fmt.Sprintf(" -Path %s", EscapePSString(path))
	} else {
		cmd += " -Path $null"
	}
	cmd += " -ErrorAction Stop"

	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("update hard drive on VM %q (%s %d:%d): %w", vmName, controllerType, controllerNumber, controllerLocation, err)
		}
		return nil
	})
}

func (c *WinRMClient) DeleteHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int) error {
	unlock := c.vmLock(vmName)
	defer unlock()

	cmd := fmt.Sprintf(
		"Remove-VMHardDiskDrive -VMName %s -ControllerType %s -ControllerNumber %d -ControllerLocation %d -ErrorAction Stop",
		EscapePSString(vmName), EscapePSString(controllerType), controllerNumber, controllerLocation,
	)
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("delete hard drive on VM %q (%s %d:%d): %w", vmName, controllerType, controllerNumber, controllerLocation, err)
		}
		return nil
	})
}

func (c *WinRMClient) ListHardDrives(ctx context.Context, vmName string) ([]HardDrive, error) {
	cmd := fmt.Sprintf(
		"Get-VMHardDiskDrive -VMName %s -ErrorAction Stop | Select-Object VMName,Path,ControllerType,ControllerNumber,ControllerLocation | ConvertTo-Json",
		EscapePSString(vmName),
	)
	stdout, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("list hard drives on VM %q: %w", vmName, err)
	}
	if stdout == "" {
		return nil, nil
	}

	// PowerShell ConvertTo-Json returns an object for single items, array for multiple
	trimmed := strings.TrimSpace(stdout)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var drives []HardDrive
		if err := json.Unmarshal([]byte(trimmed), &drives); err != nil {
			return nil, fmt.Errorf("unmarshal hard drives: %w", err)
		}
		return drives, nil
	}

	var drive HardDrive
	if err := json.Unmarshal([]byte(trimmed), &drive); err != nil {
		return nil, fmt.Errorf("unmarshal hard drive: %w", err)
	}
	return []HardDrive{drive}, nil
}
