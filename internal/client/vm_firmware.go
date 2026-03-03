package client

import (
	"context"
	"fmt"
	"time"
)

func buildSetVMFirmwareCommand(name string, opts VMFirmwareOptions) string {
	cmd := fmt.Sprintf("Set-VMFirmware -VMName %s", EscapePSString(name))
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
	return cmd
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
	return fmt.Sprintf(`$fw = Get-VMFirmware -VMName %s -ErrorAction Stop; `+
		`$fbd = $fw.BootOrder | Select-Object -First 1; `+
		`$fbdType = 'Unknown'; $fbdCN = 0; $fbdCL = 0; `+
		`if ($fbd -and $fbd.Device) { `+
		`if ($fbd.Device -is [Microsoft.HyperV.PowerShell.HardDiskDrive]) { $fbdType = 'HardDiskDrive'; $fbdCN = $fbd.Device.ControllerNumber; $fbdCL = $fbd.Device.ControllerLocation } `+
		`elseif ($fbd.Device -is [Microsoft.HyperV.PowerShell.DvdDrive]) { $fbdType = 'DvdDrive'; $fbdCN = $fbd.Device.ControllerNumber; $fbdCL = $fbd.Device.ControllerLocation } `+
		`elseif ($fbd.Device -is [Microsoft.HyperV.PowerShell.VMNetworkAdapter]) { $fbdType = 'NetworkAdapter' } `+
		`}; `+
		`[PSCustomObject]@{ `+
		`SecureBootEnabled = if ($fw.SecureBoot -eq 'On' -or $fw.SecureBoot -eq 1) { 'On' } else { 'Off' }; `+
		`SecureBootTemplate = $fw.SecureBootTemplate; `+
		`FirstBootDeviceType = $fbdType; `+
		`FirstBootDeviceControllerNumber = $fbdCN; `+
		`FirstBootDeviceControllerLocation = $fbdCL `+
		`} | ConvertTo-Json`, vmEsc)
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
