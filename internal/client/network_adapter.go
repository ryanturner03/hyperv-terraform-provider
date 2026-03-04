package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (c *WinRMClient) CreateNetworkAdapter(ctx context.Context, opts AdapterOptions) (*NetworkAdapter, error) {
	unlock := c.vmLock(opts.VMName)
	defer unlock()

	cmd := fmt.Sprintf("Add-VMNetworkAdapter -VMName %s -Name %s",
		EscapePSString(opts.VMName), EscapePSString(opts.Name))
	if opts.SwitchName != "" {
		cmd += fmt.Sprintf(" -SwitchName %s", EscapePSString(opts.SwitchName))
	}
	if opts.MacAddress != "" {
		cmd += fmt.Sprintf(" -StaticMacAddress %s", EscapePSString(opts.MacAddress))
	}
	cmd += " -ErrorAction Stop"

	err := retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("create adapter %q on VM %q: %w", opts.Name, opts.VMName, err)
	}

	if opts.VlanIDSet && opts.VlanID > 0 {
		vlanCmd := fmt.Sprintf("Set-VMNetworkAdapterVlan -VMName %s -VMNetworkAdapterName %s -Access -VlanId %d -ErrorAction Stop",
			EscapePSString(opts.VMName), EscapePSString(opts.Name), opts.VlanID)
		err = retryOnConflict(ctx, 3, 3*time.Second, func() error {
			_, _, err := c.ps.Run(ctx, vlanCmd)
			return err
		})
		if err != nil {
			_ = c.deleteNetworkAdapterNoLock(ctx, opts.VMName, opts.Name) // best-effort cleanup
			return nil, fmt.Errorf("set VLAN on adapter %q: %w", opts.Name, err)
		}
	}

	return c.GetNetworkAdapter(ctx, opts.VMName, opts.Name)
}

func (c *WinRMClient) GetNetworkAdapter(ctx context.Context, vmName, name string) (*NetworkAdapter, error) {
	cmd := fmt.Sprintf(`$adapters = @(Get-VMNetworkAdapter -VMName %s -Name %s -ErrorAction Stop); if ($adapters.Count -gt 1) { throw "found $($adapters.Count) adapters named %s on VM %s - adapter names must be unique" }; $adapter = $adapters[0]; $vlan = Get-VMNetworkAdapterVlan -VMNetworkAdapter $adapter -ErrorAction SilentlyContinue; [PSCustomObject]@{ Name = $adapter.Name; VMName = $adapter.VMName; SwitchName = $adapter.SwitchName; MacAddress = $adapter.MacAddress; DynamicMacAddressEnabled = $adapter.DynamicMacAddressEnabled; VlanID = if ($vlan) { $vlan.AccessVlanId } else { 0 } } | ConvertTo-Json`,
		EscapePSString(vmName), EscapePSString(name), EscapePSString(name), EscapePSString(vmName))
	var adapter NetworkAdapter
	err := c.ps.RunJSON(ctx, cmd, &adapter)
	if err != nil {
		return nil, fmt.Errorf("get adapter %q on VM %q: %w", name, vmName, err)
	}
	return &adapter, nil
}

func (c *WinRMClient) UpdateNetworkAdapter(ctx context.Context, vmName, name string, opts AdapterOptions) error {
	unlock := c.vmLock(vmName)
	defer unlock()

	if opts.SwitchName != "" {
		cmd := fmt.Sprintf("Connect-VMNetworkAdapter -VMName %s -Name %s -SwitchName %s -ErrorAction Stop",
			EscapePSString(vmName), EscapePSString(name), EscapePSString(opts.SwitchName))
		err := retryOnConflict(ctx, 3, 3*time.Second, func() error {
			_, _, err := c.ps.Run(ctx, cmd)
			if err != nil {
				return fmt.Errorf("connect adapter %q to switch: %w", name, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if opts.VlanIDSet && opts.VlanID > 0 {
		cmd := fmt.Sprintf("Set-VMNetworkAdapterVlan -VMName %s -VMNetworkAdapterName %s -Access -VlanId %d -ErrorAction Stop",
			EscapePSString(vmName), EscapePSString(name), opts.VlanID)
		return retryOnConflict(ctx, 3, 3*time.Second, func() error {
			_, _, err := c.ps.Run(ctx, cmd)
			if err != nil {
				return fmt.Errorf("set VLAN on adapter %q: %w", name, err)
			}
			return nil
		})
	} else if opts.VlanIDSet && opts.VlanID == 0 {
		cmd := fmt.Sprintf("Set-VMNetworkAdapterVlan -VMName %s -VMNetworkAdapterName %s -Untagged -ErrorAction Stop",
			EscapePSString(vmName), EscapePSString(name))
		return retryOnConflict(ctx, 3, 3*time.Second, func() error {
			_, _, err := c.ps.Run(ctx, cmd)
			if err != nil {
				return fmt.Errorf("clear VLAN on adapter %q: %w", name, err)
			}
			return nil
		})
	}
	return nil
}

func (c *WinRMClient) DeleteNetworkAdapter(ctx context.Context, vmName, name string) error {
	unlock := c.vmLock(vmName)
	defer unlock()

	return c.deleteNetworkAdapterNoLock(ctx, vmName, name)
}

func (c *WinRMClient) ListNetworkAdapters(ctx context.Context, vmName string) ([]NetworkAdapter, error) {
	cmd := fmt.Sprintf(
		`Get-VMNetworkAdapter -VMName %s -ErrorAction Stop | ForEach-Object { $adapter = $_; $vlan = Get-VMNetworkAdapterVlan -VMNetworkAdapter $adapter -ErrorAction SilentlyContinue; [PSCustomObject]@{ Name = $adapter.Name; VMName = $adapter.VMName; SwitchName = $adapter.SwitchName; MacAddress = $adapter.MacAddress; DynamicMacAddressEnabled = $adapter.DynamicMacAddressEnabled; VlanID = if ($vlan) { $vlan.AccessVlanId } else { 0 } } } | ConvertTo-Json`,
		EscapePSString(vmName),
	)
	stdout, _, err := c.ps.Run(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("list network adapters on VM %q: %w", vmName, err)
	}
	if stdout == "" {
		return nil, nil
	}

	// PowerShell ConvertTo-Json returns an object for single items, array for multiple
	trimmed := strings.TrimSpace(stdout)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var adapters []NetworkAdapter
		if err := json.Unmarshal([]byte(trimmed), &adapters); err != nil {
			return nil, fmt.Errorf("unmarshal network adapters: %w", err)
		}
		return adapters, nil
	}

	var adapter NetworkAdapter
	if err := json.Unmarshal([]byte(trimmed), &adapter); err != nil {
		return nil, fmt.Errorf("unmarshal network adapter: %w", err)
	}
	return []NetworkAdapter{adapter}, nil
}

func (c *WinRMClient) deleteNetworkAdapterNoLock(ctx context.Context, vmName, name string) error {
	cmd := fmt.Sprintf("Remove-VMNetworkAdapter -VMName %s -Name %s -ErrorAction Stop",
		EscapePSString(vmName), EscapePSString(name))
	return retryOnConflict(ctx, 3, 3*time.Second, func() error {
		_, _, err := c.ps.Run(ctx, cmd)
		if err != nil {
			return fmt.Errorf("delete adapter %q on VM %q: %w", name, vmName, err)
		}
		return nil
	})
}
