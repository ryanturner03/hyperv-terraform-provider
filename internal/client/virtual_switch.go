package client

import (
	"context"
	"fmt"
)

func (c *WinRMClient) CreateVirtualSwitch(ctx context.Context, opts SwitchOptions) (*VirtualSwitch, error) {
	cmd := fmt.Sprintf("New-VMSwitch -Name %s", EscapePSString(opts.Name))
	switch opts.SwitchType {
	case "External":
		cmd += fmt.Sprintf(" -NetAdapterName %s", EscapePSString(opts.NetAdapterName))
		if opts.AllowManagementOSSet {
			if opts.AllowManagementOS {
				cmd += " -AllowManagementOS $true"
			} else {
				cmd += " -AllowManagementOS $false"
			}
		}
	case "Internal":
		cmd += " -SwitchType Internal"
	case "Private":
		cmd += " -SwitchType Private"
	}
	cmd += " -ErrorAction Stop"

	_, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("create switch %q: %w", opts.Name, err)
	}
	return c.GetVirtualSwitch(ctx, opts.Name)
}

func (c *WinRMClient) GetVirtualSwitch(ctx context.Context, name string) (*VirtualSwitch, error) {
	cmd := fmt.Sprintf("Get-VMSwitch -Name %s -ErrorAction Stop | Select-Object Name,SwitchType,NetAdapterInterfaceDescription,AllowManagementOS | ConvertTo-Json", EscapePSString(name))
	var sw VirtualSwitch
	err := c.ps.RunJSON(ctx, cmd, &sw)
	if err != nil {
		return nil, fmt.Errorf("get switch %q: %w", name, err)
	}
	return &sw, nil
}

func (c *WinRMClient) UpdateVirtualSwitch(ctx context.Context, name string, opts SwitchOptions) error {
	cmd := fmt.Sprintf("Set-VMSwitch -Name %s", EscapePSString(name))
	if opts.NetAdapterName != "" {
		cmd += fmt.Sprintf(" -NetAdapterName %s", EscapePSString(opts.NetAdapterName))
	}
	if opts.AllowManagementOSSet {
		if opts.AllowManagementOS {
			cmd += " -AllowManagementOS $true"
		} else {
			cmd += " -AllowManagementOS $false"
		}
	}
	cmd += " -ErrorAction Stop"
	_, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("update switch %q: %w", name, err)
	}
	return nil
}

func (c *WinRMClient) DeleteVirtualSwitch(ctx context.Context, name string) error {
	cmd := fmt.Sprintf("Remove-VMSwitch -Name %s -Force -ErrorAction Stop", EscapePSString(name))
	_, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return fmt.Errorf("delete switch %q: %w", name, err)
	}
	return nil
}
