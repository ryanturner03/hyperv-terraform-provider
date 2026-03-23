package client

import (
	"context"
	"fmt"
	"time"
)

func buildNewVMCommand(opts VMOptions) string {
	cmd := fmt.Sprintf("New-VM -Name %s -Generation %d -MemoryStartupBytes %d -NoVHD -ErrorAction Stop",
		EscapePSString(opts.Name), opts.Generation, opts.MemoryStartupBytes)
	return cmd
}

func buildSetVMCommand(opts VMOptions) string {
	cmd := fmt.Sprintf("Set-VM -Name %s", EscapePSString(opts.Name))
	if opts.ProcessorCount > 0 {
		cmd += fmt.Sprintf(" -ProcessorCount %d", opts.ProcessorCount)
	}
	if opts.MemoryStartupBytes > 0 {
		cmd += fmt.Sprintf(" -MemoryStartupBytes %d", opts.MemoryStartupBytes)
	}
	if opts.DynamicMemory {
		cmd += " -DynamicMemory:$true"
		if opts.MemoryMinimumBytes > 0 {
			cmd += fmt.Sprintf(" -MemoryMinimumBytes %d", opts.MemoryMinimumBytes)
		}
		if opts.MemoryMaximumBytes > 0 {
			cmd += fmt.Sprintf(" -MemoryMaximumBytes %d", opts.MemoryMaximumBytes)
		}
	} else {
		cmd += " -DynamicMemory:$false"
	}
	if opts.Notes != "" {
		cmd += fmt.Sprintf(" -Notes %s", EscapePSString(opts.Notes))
	}
	if opts.AutomaticStartAction != "" {
		cmd += fmt.Sprintf(" -AutomaticStartAction %s", EscapePSString(opts.AutomaticStartAction))
	}
	if opts.AutomaticStopAction != "" {
		cmd += fmt.Sprintf(" -AutomaticStopAction %s", EscapePSString(opts.AutomaticStopAction))
	}
	if opts.CheckpointType != "" {
		cmd += fmt.Sprintf(" -CheckpointType %s", EscapePSString(opts.CheckpointType))
	}
	cmd += " -ErrorAction Stop"
	return cmd
}

func buildGetVMCommand(name string) string {
	return fmt.Sprintf(`Get-VM -Name %s -ErrorAction Stop | ForEach-Object { [PSCustomObject]@{ Name = $_.Name; Generation = $_.Generation; ProcessorCount = $_.ProcessorCount; MemoryStartup = $_.MemoryStartup; MemoryMinimum = $_.MemoryMinimum; MemoryMaximum = $_.MemoryMaximum; DynamicMemoryEnabled = $_.DynamicMemoryEnabled; State = $_.State.ToString(); Notes = $_.Notes; AutomaticStartAction = $_.AutomaticStartAction.ToString(); AutomaticStopAction = $_.AutomaticStopAction.ToString(); CheckpointType = $_.CheckpointType.ToString() } } | ConvertTo-Json`, EscapePSString(name))
}

func (c *WinRMClient) CreateVM(ctx context.Context, opts VMOptions) (*VM, error) {
	unlock := c.vmLock(opts.Name)
	defer unlock()

	_, _, err := c.ps.Run(ctx, buildNewVMCommand(opts))
	if err != nil {
		return nil, fmt.Errorf("create VM %q: %w", opts.Name, err)
	}

	// Remove the default network adapter created by New-VM so that only
	// Terraform-managed hyperv_network_adapter resources define NICs.
	removeNIC := fmt.Sprintf("Remove-VMNetworkAdapter -VMName %s -ErrorAction SilentlyContinue", EscapePSString(opts.Name))
	_, _, _ = c.ps.Run(ctx, removeNIC)

	err = retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, buildSetVMCommand(opts))
		return err
	})
	if err != nil {
		_ = c.deleteVMNoLock(ctx, opts.Name) // best-effort cleanup of orphan
		return nil, fmt.Errorf("configure VM %q: %w", opts.Name, err)
	}

	return c.GetVM(ctx, opts.Name)
}

func (c *WinRMClient) GetVM(ctx context.Context, name string) (*VM, error) {
	var vm VM
	err := c.ps.RunJSON(ctx, buildGetVMCommand(name), &vm)
	if err != nil {
		return nil, fmt.Errorf("get VM %q: %w", name, err)
	}
	return &vm, nil
}

func (c *WinRMClient) UpdateVM(ctx context.Context, name string, opts VMOptions) error {
	unlock := c.vmLock(name)
	defer unlock()

	opts.Name = name
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, buildSetVMCommand(opts))
		if err != nil {
			return fmt.Errorf("update VM %q: %w", name, err)
		}
		return nil
	})
}

func (c *WinRMClient) DeleteVM(ctx context.Context, name string) error {
	unlock := c.vmLock(name)
	defer unlock()

	return c.deleteVMNoLock(ctx, name)
}

func (c *WinRMClient) deleteVMNoLock(ctx context.Context, name string) error {
	// Stop if running, then remove
	stopCmd := fmt.Sprintf("$vm = Get-VM -Name %s -ErrorAction SilentlyContinue; if ($vm -and $vm.State -ne 'Off') { Stop-VM -Name %s -Force -TurnOff -ErrorAction Stop }",
		EscapePSString(name), EscapePSString(name))
	err := retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, stopCmd)
		if err != nil {
			return fmt.Errorf("stop VM %q: %w", name, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	removeCmd := fmt.Sprintf("Remove-VM -Name %s -Force -ErrorAction Stop", EscapePSString(name))
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, removeCmd)
		if err != nil {
			return fmt.Errorf("remove VM %q: %w", name, err)
		}
		return nil
	})
}

func (c *WinRMClient) SetVMState(ctx context.Context, name string, state VMState) error {
	unlock := c.vmLock(name)
	defer unlock()

	var cmd string
	switch state {
	case VMStateRunning:
		// Grant the VM's security principal full control on all attached VHDs
		// before starting. This is necessary because VHD replacement (destroy +
		// recreate) produces a new file that lacks the ACL entry Hyper-V set
		// when the drive was first attached.
		if err := c.grantVMStorageAccess(ctx, name); err != nil {
			return fmt.Errorf("grant VM %q storage access: %w", name, err)
		}
		cmd = fmt.Sprintf("Start-VM -Name %s -ErrorAction Stop", EscapePSString(name))
	case VMStateOff:
		cmd = fmt.Sprintf("Stop-VM -Name %s -Force -TurnOff -ErrorAction Stop", EscapePSString(name))
	default:
		return fmt.Errorf("unsupported VM state: %s", state)
	}
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("set VM %q state to %s: %w", name, state, err)
		}
		return nil
	})
}

// grantVMStorageAccess grants the VM's security principal (NT VIRTUAL MACHINE\{VMId})
// full control on all VHD files attached to the VM. This ensures the VM can access
// VHDs that were replaced (destroyed and recreated) after the initial attachment.
func (c *WinRMClient) grantVMStorageAccess(ctx context.Context, name string) error {
	_, _, err := c.ps.Run(ctx, buildGrantVMStorageAccessCommand(name))
	return err
}

func buildGrantVMStorageAccessCommand(name string) string {
	return fmt.Sprintf(`$vmId = (Get-VM -Name %[1]s -ErrorAction Stop).VMId
Get-VMHardDiskDrive -VMName %[1]s -ErrorAction SilentlyContinue | Where-Object { $_.Path } | ForEach-Object {
    $acl = Get-Acl -LiteralPath $_.Path
    $rule = New-Object System.Security.AccessControl.FileSystemAccessRule(
        "NT VIRTUAL MACHINE\$vmId", 'FullControl', 'Allow')
    $acl.SetAccessRule($rule)
    Set-Acl -LiteralPath $_.Path -AclObject $acl
}`, EscapePSString(name))
}
