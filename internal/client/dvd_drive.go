package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (c *WinRMClient) CreateDVDDrive(ctx context.Context, opts DVDDriveOptions) (*DVDDrive, error) {
	unlock := c.vmLock(opts.VMName)
	defer unlock()

	cmd := fmt.Sprintf("Add-VMDvdDrive -VMName %s -ControllerNumber %d",
		EscapePSString(opts.VMName), opts.ControllerNumber)
	if opts.ControllerLocationSet {
		cmd += fmt.Sprintf(" -ControllerLocation %d", opts.ControllerLocation)
	}
	if opts.Path != "" {
		cmd += fmt.Sprintf(" -Path %s", EscapePSString(opts.Path))
	}
	cmd += " -Passthru -ErrorAction Stop | Select-Object VMName,Path,ControllerNumber,ControllerLocation | ConvertTo-Json"

	var drive DVDDrive
	err := retryOnConflict(ctx, 3, 3*time.Second, func() error {
		return c.ps.RunJSON(ctx, cmd, &drive)
	})
	if err != nil {
		return nil, fmt.Errorf("create DVD drive on VM %q: %w", opts.VMName, err)
	}
	return &drive, nil
}

func (c *WinRMClient) GetDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int) (*DVDDrive, error) {
	cmd := fmt.Sprintf(
		"Get-VMDvdDrive -VMName %s -ControllerNumber %d -ControllerLocation %d -ErrorAction Stop | Select-Object VMName,Path,ControllerNumber,ControllerLocation | ConvertTo-Json",
		EscapePSString(vmName), controllerNumber, controllerLocation,
	)
	var drive DVDDrive
	err := c.ps.RunJSON(ctx, cmd, &drive)
	if err != nil {
		return nil, fmt.Errorf("get DVD drive on VM %q (controller %d:%d): %w", vmName, controllerNumber, controllerLocation, err)
	}
	return &drive, nil
}

func (c *WinRMClient) UpdateDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int, opts DVDDriveOptions) error {
	unlock := c.vmLock(vmName)
	defer unlock()

	cmd := fmt.Sprintf(
		"Set-VMDvdDrive -VMName %s -ControllerNumber %d -ControllerLocation %d",
		EscapePSString(vmName), controllerNumber, controllerLocation,
	)
	if opts.Path != "" {
		cmd += fmt.Sprintf(" -Path %s", EscapePSString(opts.Path))
	} else {
		cmd += " -Path $null"
	}
	cmd += " -ErrorAction Stop"

	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("update DVD drive on VM %q (controller %d:%d): %w", vmName, controllerNumber, controllerLocation, err)
		}
		return nil
	})
}

func (c *WinRMClient) DeleteDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int) error {
	unlock := c.vmLock(vmName)
	defer unlock()

	cmd := fmt.Sprintf(
		"Remove-VMDvdDrive -VMName %s -ControllerNumber %d -ControllerLocation %d -ErrorAction Stop",
		EscapePSString(vmName), controllerNumber, controllerLocation,
	)
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("delete DVD drive on VM %q (controller %d:%d): %w", vmName, controllerNumber, controllerLocation, err)
		}
		return nil
	})
}

func (c *WinRMClient) ListDVDDrives(ctx context.Context, vmName string) ([]DVDDrive, error) {
	cmd := fmt.Sprintf(
		"Get-VMDvdDrive -VMName %s -ErrorAction Stop | Select-Object VMName,Path,ControllerNumber,ControllerLocation | ConvertTo-Json",
		EscapePSString(vmName),
	)
	stdout, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("list DVD drives on VM %q: %w", vmName, err)
	}
	if stdout == "" {
		return nil, nil
	}

	// PowerShell ConvertTo-Json returns an object for single items, array for multiple
	trimmed := strings.TrimSpace(stdout)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var drives []DVDDrive
		if err := json.Unmarshal([]byte(trimmed), &drives); err != nil {
			return nil, fmt.Errorf("unmarshal DVD drives: %w", err)
		}
		return drives, nil
	}

	var drive DVDDrive
	if err := json.Unmarshal([]byte(trimmed), &drive); err != nil {
		return nil, fmt.Errorf("unmarshal DVD drive: %w", err)
	}
	return []DVDDrive{drive}, nil
}
