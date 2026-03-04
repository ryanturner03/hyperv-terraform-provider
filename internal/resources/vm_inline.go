package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

// Inline block models — no vm_name field, inherited from parent VM.

type inlineHardDriveModel struct {
	Path               types.String `tfsdk:"path"`
	ControllerType     types.String `tfsdk:"controller_type"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
}

type inlineDVDDriveModel struct {
	Path               types.String `tfsdk:"path"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
}

type inlineNetworkAdapterModel struct {
	Name              types.String `tfsdk:"name"`
	SwitchName        types.String `tfsdk:"switch_name"`
	VlanID            types.Int64  `tfsdk:"vlan_id"`
	MacAddress        types.String `tfsdk:"mac_address"`
	DynamicMacAddress types.Bool   `tfsdk:"dynamic_mac_address"`
}

// Attr type maps for constructing types.List values.

func inlineHardDriveAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"path":                types.StringType,
		"controller_type":     types.StringType,
		"controller_number":   types.Int64Type,
		"controller_location": types.Int64Type,
	}
}

func inlineDVDDriveAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"path":                types.StringType,
		"controller_number":   types.Int64Type,
		"controller_location": types.Int64Type,
	}
}

func inlineNetworkAdapterAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":                types.StringType,
		"switch_name":         types.StringType,
		"vlan_id":             types.Int64Type,
		"mac_address":         types.StringType,
		"dynamic_mac_address": types.BoolType,
	}
}

// Create helpers — create inline sub-resources and return models with computed fields populated.

func createInlineHardDrives(ctx context.Context, c client.HyperVClient, vmName string, planned []inlineHardDriveModel) ([]inlineHardDriveModel, error) {
	var result []inlineHardDriveModel
	for _, p := range planned {
		opts := client.HardDriveOptions{
			VMName:         vmName,
			Path:           p.Path.ValueString(),
			ControllerType: p.ControllerType.ValueString(),
			ControllerNumber: int(p.ControllerNumber.ValueInt64()),
		}
		if !p.ControllerLocation.IsNull() && !p.ControllerLocation.IsUnknown() {
			opts.ControllerLocation = int(p.ControllerLocation.ValueInt64())
			opts.ControllerLocationSet = true
		}

		drive, err := c.CreateHardDrive(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("create inline hard drive: %w", err)
		}

		ctStr := "SCSI"
		if drive.ControllerType == 0 {
			ctStr = "IDE"
		}
		result = append(result, inlineHardDriveModel{
			Path:               types.StringValue(drive.Path),
			ControllerType:     types.StringValue(ctStr),
			ControllerNumber:   types.Int64Value(int64(drive.ControllerNumber)),
			ControllerLocation: types.Int64Value(int64(drive.ControllerLocation)),
		})
	}
	return result, nil
}

func createInlineDVDDrives(ctx context.Context, c client.HyperVClient, vmName string, planned []inlineDVDDriveModel) ([]inlineDVDDriveModel, error) {
	var result []inlineDVDDriveModel
	for _, p := range planned {
		opts := client.DVDDriveOptions{
			VMName:           vmName,
			Path:             p.Path.ValueString(),
			ControllerNumber: int(p.ControllerNumber.ValueInt64()),
		}
		if !p.ControllerLocation.IsNull() && !p.ControllerLocation.IsUnknown() {
			opts.ControllerLocation = int(p.ControllerLocation.ValueInt64())
			opts.ControllerLocationSet = true
		}

		drive, err := c.CreateDVDDrive(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("create inline DVD drive: %w", err)
		}

		result = append(result, inlineDVDDriveModel{
			Path:               types.StringValue(drive.Path),
			ControllerNumber:   types.Int64Value(int64(drive.ControllerNumber)),
			ControllerLocation: types.Int64Value(int64(drive.ControllerLocation)),
		})
	}
	return result, nil
}

func createInlineNetworkAdapters(ctx context.Context, c client.HyperVClient, vmName string, planned []inlineNetworkAdapterModel) ([]inlineNetworkAdapterModel, error) {
	var result []inlineNetworkAdapterModel
	for _, p := range planned {
		opts := client.AdapterOptions{
			VMName:     vmName,
			Name:       p.Name.ValueString(),
			SwitchName: p.SwitchName.ValueString(),
			DynamicMacAddress: p.DynamicMacAddress.ValueBool(),
		}
		if !p.MacAddress.IsNull() && !p.MacAddress.IsUnknown() && p.MacAddress.ValueString() != "" {
			opts.MacAddress = p.MacAddress.ValueString()
		}
		if !p.VlanID.IsNull() && !p.VlanID.IsUnknown() {
			opts.VlanID = int(p.VlanID.ValueInt64())
			opts.VlanIDSet = true
		}

		adapter, err := c.CreateNetworkAdapter(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("create inline network adapter %q: %w", opts.Name, err)
		}

		result = append(result, inlineNetworkAdapterModel{
			Name:              types.StringValue(adapter.Name),
			SwitchName:        types.StringValue(adapter.SwitchName),
			VlanID:            types.Int64Value(int64(adapter.VlanID)),
			MacAddress:        types.StringValue(adapter.MacAddress),
			DynamicMacAddress: types.BoolValue(adapter.DynamicMacAddress),
		})
	}
	return result, nil
}

// List converters — convert model slices to types.List for state.

func hardDrivesToList(ctx context.Context, drives []inlineHardDriveModel, diags *diag.Diagnostics) types.List {
	if drives == nil {
		return types.ListNull(types.ObjectType{AttrTypes: inlineHardDriveAttrTypes()})
	}
	list, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: inlineHardDriveAttrTypes()}, drives)
	diags.Append(d...)
	return list
}

func dvdDrivesToList(ctx context.Context, drives []inlineDVDDriveModel, diags *diag.Diagnostics) types.List {
	if drives == nil {
		return types.ListNull(types.ObjectType{AttrTypes: inlineDVDDriveAttrTypes()})
	}
	list, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: inlineDVDDriveAttrTypes()}, drives)
	diags.Append(d...)
	return list
}

func networkAdaptersToList(ctx context.Context, adapters []inlineNetworkAdapterModel, diags *diag.Diagnostics) types.List {
	if adapters == nil {
		return types.ListNull(types.ObjectType{AttrTypes: inlineNetworkAdapterAttrTypes()})
	}
	list, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: inlineNetworkAdapterAttrTypes()}, adapters)
	diags.Append(d...)
	return list
}

// Read helpers — convert client structs to inline models.

func hardDrivesFromClient(drives []client.HardDrive) []inlineHardDriveModel {
	if len(drives) == 0 {
		return nil
	}
	var result []inlineHardDriveModel
	for _, d := range drives {
		ctStr := "SCSI"
		if d.ControllerType == 0 {
			ctStr = "IDE"
		}
		result = append(result, inlineHardDriveModel{
			Path:               types.StringValue(d.Path),
			ControllerType:     types.StringValue(ctStr),
			ControllerNumber:   types.Int64Value(int64(d.ControllerNumber)),
			ControllerLocation: types.Int64Value(int64(d.ControllerLocation)),
		})
	}
	return result
}

func dvdDrivesFromClient(drives []client.DVDDrive) []inlineDVDDriveModel {
	if len(drives) == 0 {
		return nil
	}
	var result []inlineDVDDriveModel
	for _, d := range drives {
		result = append(result, inlineDVDDriveModel{
			Path:               types.StringValue(d.Path),
			ControllerNumber:   types.Int64Value(int64(d.ControllerNumber)),
			ControllerLocation: types.Int64Value(int64(d.ControllerLocation)),
		})
	}
	return result
}

func networkAdaptersFromClient(adapters []client.NetworkAdapter) []inlineNetworkAdapterModel {
	if len(adapters) == 0 {
		return nil
	}
	var result []inlineNetworkAdapterModel
	for _, a := range adapters {
		result = append(result, inlineNetworkAdapterModel{
			Name:              types.StringValue(a.Name),
			SwitchName:        types.StringValue(a.SwitchName),
			VlanID:            types.Int64Value(int64(a.VlanID)),
			MacAddress:        types.StringValue(a.MacAddress),
			DynamicMacAddress: types.BoolValue(a.DynamicMacAddress),
		})
	}
	return result
}

// Diff helpers — compare old/new lists and apply creates/updates/deletes.

func diffAndApplyHardDrives(ctx context.Context, c client.HyperVClient, vmName string, oldList, newList []inlineHardDriveModel) error {
	type hdKey struct {
		ct   string
		cn   int64
		cl   int64
	}

	oldMap := make(map[hdKey]inlineHardDriveModel)
	for _, o := range oldList {
		k := hdKey{o.ControllerType.ValueString(), o.ControllerNumber.ValueInt64(), o.ControllerLocation.ValueInt64()}
		oldMap[k] = o
	}

	newMap := make(map[hdKey]inlineHardDriveModel)
	for _, n := range newList {
		k := hdKey{n.ControllerType.ValueString(), n.ControllerNumber.ValueInt64(), n.ControllerLocation.ValueInt64()}
		newMap[k] = n
	}

	// Delete items in old but not in new
	for k, o := range oldMap {
		if _, exists := newMap[k]; !exists {
			if err := c.DeleteHardDrive(ctx, vmName, o.ControllerType.ValueString(), int(o.ControllerNumber.ValueInt64()), int(o.ControllerLocation.ValueInt64())); err != nil {
				return fmt.Errorf("delete hard drive %s %d:%d: %w", k.ct, k.cn, k.cl, err)
			}
		}
	}

	// Create or update
	for k, n := range newMap {
		if o, exists := oldMap[k]; exists {
			// Update path if changed
			if o.Path.ValueString() != n.Path.ValueString() {
				if err := c.UpdateHardDrive(ctx, vmName, n.ControllerType.ValueString(), int(n.ControllerNumber.ValueInt64()), int(n.ControllerLocation.ValueInt64()), n.Path.ValueString()); err != nil {
					return fmt.Errorf("update hard drive %s %d:%d: %w", k.ct, k.cn, k.cl, err)
				}
			}
		} else {
			// Create
			opts := client.HardDriveOptions{
				VMName:                vmName,
				Path:                  n.Path.ValueString(),
				ControllerType:        n.ControllerType.ValueString(),
				ControllerNumber:      int(n.ControllerNumber.ValueInt64()),
				ControllerLocation:    int(n.ControllerLocation.ValueInt64()),
				ControllerLocationSet: true,
			}
			if _, err := c.CreateHardDrive(ctx, opts); err != nil {
				return fmt.Errorf("create hard drive %s %d:%d: %w", k.ct, k.cn, k.cl, err)
			}
		}
	}

	return nil
}

func diffAndApplyDVDDrives(ctx context.Context, c client.HyperVClient, vmName string, oldList, newList []inlineDVDDriveModel) error {
	type dvdKey struct {
		cn int64
		cl int64
	}

	oldMap := make(map[dvdKey]inlineDVDDriveModel)
	for _, o := range oldList {
		k := dvdKey{o.ControllerNumber.ValueInt64(), o.ControllerLocation.ValueInt64()}
		oldMap[k] = o
	}

	newMap := make(map[dvdKey]inlineDVDDriveModel)
	for _, n := range newList {
		k := dvdKey{n.ControllerNumber.ValueInt64(), n.ControllerLocation.ValueInt64()}
		newMap[k] = n
	}

	// Delete items in old but not in new
	for k := range oldMap {
		if _, exists := newMap[k]; !exists {
			if err := c.DeleteDVDDrive(ctx, vmName, int(k.cn), int(k.cl)); err != nil {
				return fmt.Errorf("delete DVD drive %d:%d: %w", k.cn, k.cl, err)
			}
		}
	}

	// Create or update
	for k, n := range newMap {
		if o, exists := oldMap[k]; exists {
			// Update path if changed
			if o.Path.ValueString() != n.Path.ValueString() {
				opts := client.DVDDriveOptions{
					VMName:           vmName,
					Path:             n.Path.ValueString(),
					ControllerNumber: int(n.ControllerNumber.ValueInt64()),
				}
				if err := c.UpdateDVDDrive(ctx, vmName, int(n.ControllerNumber.ValueInt64()), int(n.ControllerLocation.ValueInt64()), opts); err != nil {
					return fmt.Errorf("update DVD drive %d:%d: %w", k.cn, k.cl, err)
				}
			}
		} else {
			// Create
			opts := client.DVDDriveOptions{
				VMName:                vmName,
				Path:                  n.Path.ValueString(),
				ControllerNumber:      int(n.ControllerNumber.ValueInt64()),
				ControllerLocation:    int(n.ControllerLocation.ValueInt64()),
				ControllerLocationSet: true,
			}
			if _, err := c.CreateDVDDrive(ctx, opts); err != nil {
				return fmt.Errorf("create DVD drive %d:%d: %w", k.cn, k.cl, err)
			}
		}
	}

	return nil
}

func diffAndApplyNetworkAdapters(ctx context.Context, c client.HyperVClient, vmName string, oldList, newList []inlineNetworkAdapterModel) error {
	oldMap := make(map[string]inlineNetworkAdapterModel)
	for _, o := range oldList {
		oldMap[o.Name.ValueString()] = o
	}

	newMap := make(map[string]inlineNetworkAdapterModel)
	for _, n := range newList {
		newMap[n.Name.ValueString()] = n
	}

	// Delete adapters in old but not in new
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			if err := c.DeleteNetworkAdapter(ctx, vmName, name); err != nil {
				return fmt.Errorf("delete network adapter %q: %w", name, err)
			}
		}
	}

	// Create or update
	for name, n := range newMap {
		if o, exists := oldMap[name]; exists {
			// Update mutable fields if changed
			needsUpdate := false
			opts := client.AdapterOptions{
				VMName: vmName,
				Name:   name,
			}
			if o.SwitchName.ValueString() != n.SwitchName.ValueString() {
				opts.SwitchName = n.SwitchName.ValueString()
				needsUpdate = true
			}
			if o.VlanID.ValueInt64() != n.VlanID.ValueInt64() {
				opts.VlanID = int(n.VlanID.ValueInt64())
				opts.VlanIDSet = true
				needsUpdate = true
			}
			if needsUpdate {
				if err := c.UpdateNetworkAdapter(ctx, vmName, name, opts); err != nil {
					return fmt.Errorf("update network adapter %q: %w", name, err)
				}
			}
		} else {
			// Create
			opts := client.AdapterOptions{
				VMName:            vmName,
				Name:              name,
				SwitchName:        n.SwitchName.ValueString(),
				DynamicMacAddress: n.DynamicMacAddress.ValueBool(),
			}
			if !n.MacAddress.IsNull() && !n.MacAddress.IsUnknown() && n.MacAddress.ValueString() != "" {
				opts.MacAddress = n.MacAddress.ValueString()
			}
			if !n.VlanID.IsNull() && !n.VlanID.IsUnknown() {
				opts.VlanID = int(n.VlanID.ValueInt64())
				opts.VlanIDSet = true
			}
			if _, err := c.CreateNetworkAdapter(ctx, opts); err != nil {
				return fmt.Errorf("create network adapter %q: %w", name, err)
			}
		}
	}

	return nil
}
