package client

import (
	"context"
	"fmt"
	"time"
)

func buildSetVMFirmwareCommand(name string, opts VMFirmwareOptions) string {
	vmEsc := EscapePSString(name)

	// When a first boot device is specified, look it up first so we can
	// pass it to the same Set-VMFirmware call.  Combining everything into
	// a single call avoids the problem where separate Set-VMFirmware
	// invocations reset each other's settings (e.g. setting Secure Boot
	// resets the boot order and vice-versa).
	var prefix string
	if opts.FirstBootDevice != nil {
		d := opts.FirstBootDevice
		switch d.DeviceType {
		case "HardDiskDrive":
			prefix = fmt.Sprintf(
				"$dev = Get-VMHardDiskDrive -VMName %s | Where-Object { $_.ControllerNumber -eq %d -and $_.ControllerLocation -eq %d }; ",
				vmEsc, d.ControllerNumber, d.ControllerLocation)
		case "DvdDrive":
			prefix = fmt.Sprintf(
				"$dev = Get-VMDvdDrive -VMName %s | Where-Object { $_.ControllerNumber -eq %d -and $_.ControllerLocation -eq %d }; ",
				vmEsc, d.ControllerNumber, d.ControllerLocation)
		case "NetworkAdapter":
			prefix = fmt.Sprintf(
				"$dev = Get-VMNetworkAdapter -VMName %s; ", vmEsc)
		default:
			prefix = fmt.Sprintf(
				"$dev = Get-VMHardDiskDrive -VMName %s | Where-Object { $_.ControllerNumber -eq %d -and $_.ControllerLocation -eq %d }; ",
				vmEsc, d.ControllerNumber, d.ControllerLocation)
		}
		prefix += fmt.Sprintf(
			"if (-not $dev) { throw 'drive at controller %d:%d not attached yet - ensure drive resources are created first, or create VM with state=Off and update to Running after drives exist' }; ",
			d.ControllerNumber, d.ControllerLocation)
	}

	cmd := fmt.Sprintf("Set-VMFirmware -VMName %s", vmEsc)
	if opts.FirstBootDevice != nil {
		cmd += " -FirstBootDevice $dev"
	}
	if opts.SecureBootEnabled != nil {
		if *opts.SecureBootEnabled {
			cmd += " -EnableSecureBoot On"
		} else {
			cmd += " -EnableSecureBoot Off"
		}
	}
	if opts.SecureBootTemplate != "" {
		cmd += fmt.Sprintf(" -SecureBootTemplate %s", EscapePSString(opts.SecureBootTemplate))
	}
	cmd += " -ErrorAction Stop"
	return prefix + cmd
}

func buildSetVMFirstBootDeviceCommand(name string, device BootDevice) string {
	vmEsc := EscapePSString(name)

	var deviceLookup string
	switch device.DeviceType {
	case "HardDiskDrive":
		deviceLookup = fmt.Sprintf(
			"$dev = Get-VMHardDiskDrive -VMName %s | Where-Object { $_.ControllerNumber -eq %d -and $_.ControllerLocation -eq %d }",
			vmEsc, device.ControllerNumber, device.ControllerLocation)
	case "DvdDrive":
		deviceLookup = fmt.Sprintf(
			"$dev = Get-VMDvdDrive -VMName %s | Where-Object { $_.ControllerNumber -eq %d -and $_.ControllerLocation -eq %d }",
			vmEsc, device.ControllerNumber, device.ControllerLocation)
	case "NetworkAdapter":
		deviceLookup = fmt.Sprintf(
			"$dev = Get-VMNetworkAdapter -VMName %s", vmEsc)
	default:
		deviceLookup = fmt.Sprintf(
			"$dev = Get-VMHardDiskDrive -VMName %s | Where-Object { $_.ControllerNumber -eq %d -and $_.ControllerLocation -eq %d }",
			vmEsc, device.ControllerNumber, device.ControllerLocation)
	}

	return fmt.Sprintf(
		"%s; if (-not $dev) { throw 'drive at controller %d:%d not attached yet - ensure drive resources are created first, or create VM with state=Off and update to Running after drives exist' }; Set-VMFirmware -VMName %s -FirstBootDevice $dev -ErrorAction Stop",
		deviceLookup, device.ControllerNumber, device.ControllerLocation, vmEsc)
}

func buildGetVMFirmwareCommand(name string) string {
	vmEsc := EscapePSString(name)
	// Determine the first boot device by matching the BootOrder entry against
	// the VM's drives and adapters. We avoid -is type checks because objects
	// are deserialized over WinRM (Deserialized.X doesn't match X).
	return fmt.Sprintf(`$fw = Get-VMFirmware -VMName %s -ErrorAction Stop; `+
		`$fbd = $fw.BootOrder | Select-Object -First 1; `+
		`$fbdType = 'Unknown'; $fbdCN = 0; $fbdCL = 0; `+
		`if ($fbd -and $fbd.Device) { `+
		`$d = $fbd.Device; `+
		`$hdds = @(Get-VMHardDiskDrive -VMName %s -ErrorAction SilentlyContinue); `+
		`$dvds = @(Get-VMDvdDrive -VMName %s -ErrorAction SilentlyContinue); `+
		`$matched = $false; `+
		`foreach ($h in $hdds) { if ($h.Id -eq $d.Id) { $fbdType = 'HardDiskDrive'; $fbdCN = $h.ControllerNumber; $fbdCL = $h.ControllerLocation; $matched = $true; break } }; `+
		`if (-not $matched) { foreach ($dv in $dvds) { if ($dv.Id -eq $d.Id) { $fbdType = 'DvdDrive'; $fbdCN = $dv.ControllerNumber; $fbdCL = $dv.ControllerLocation; $matched = $true; break } } }; `+
		`if (-not $matched -and $d.PSObject.Properties['SwitchName']) { $fbdType = 'NetworkAdapter' } `+
		`}; `+
		`[PSCustomObject]@{ `+
		`SecureBootEnabled = $fw.SecureBoot.ToString(); `+
		`SecureBootTemplate = $fw.SecureBootTemplate; `+
		`FirstBootDeviceType = $fbdType; `+
		`FirstBootDeviceControllerNumber = $fbdCN; `+
		`FirstBootDeviceControllerLocation = $fbdCL `+
		`} | ConvertTo-Json`, vmEsc, vmEsc, vmEsc)
}

func (c *WinRMClient) SetVMFirmware(ctx context.Context, name string, opts VMFirmwareOptions) error {
	unlock := c.vmLock(name)
	defer unlock()

	cmd := buildSetVMFirmwareCommand(name, opts)
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("set firmware on VM %q: %w", name, err)
		}
		return nil
	})
}

func (c *WinRMClient) GetVMFirmware(ctx context.Context, name string) (*VMFirmware, error) {
	var fw VMFirmware
	err := c.ps.RunJSON(ctx, buildGetVMFirmwareCommand(name), &fw)
	if err != nil {
		return nil, fmt.Errorf("get firmware for VM %q: %w", name, err)
	}
	return &fw, nil
}

func (c *WinRMClient) SetVMFirstBootDevice(ctx context.Context, name string, device BootDevice) error {
	unlock := c.vmLock(name)
	defer unlock()

	cmd := buildSetVMFirstBootDeviceCommand(name, device)
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("set first boot device on VM %q: %w", name, err)
		}
		return nil
	})
}
