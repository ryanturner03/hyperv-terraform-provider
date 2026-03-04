package client

import (
	"context"
	"fmt"
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

	adapter, err := c.GetNetworkAdapter(ctx, opts.VMName, opts.Name)
	if err != nil {
		return nil, err
	}

	// Hyper-V may not assign a dynamic MAC immediately after creation.
	// Use opts.MacAddress (our input) instead of adapter.DynamicMacAddress
	// (read-back) because DynamicMacAddressEnabled can briefly be false
	// right after creation, which would skip the retry entirely.
	if opts.MacAddress == "" && (adapter.MacAddress == "" || adapter.MacAddress == "000000000000") {
		for i := 0; i < 10; i++ {
			time.Sleep(2 * time.Second)
			adapter, err = c.GetNetworkAdapter(ctx, opts.VMName, opts.Name)
			if err != nil {
				return nil, err
			}
			if adapter.MacAddress != "" && adapter.MacAddress != "000000000000" {
				break
			}
		}
	}

	return adapter, nil
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
