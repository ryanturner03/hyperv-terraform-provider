package resources

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

// mockHyperVClient implements client.HyperVClient for VM create tests.
// Unoverridden methods panic via the embedded nil interface.
type mockHyperVClient struct {
	client.HyperVClient

	createVMFn             func(ctx context.Context, opts client.VMOptions) (*client.VM, error)
	getVMFn                func(ctx context.Context, name string) (*client.VM, error)
	deleteVMFn             func(ctx context.Context, name string) error
	setVMStateFn           func(ctx context.Context, name string, state client.VMState) error
	setVMFirmwareFn        func(ctx context.Context, name string, opts client.VMFirmwareOptions) error
	getVMFirmwareFn        func(ctx context.Context, name string) (*client.VMFirmware, error)
	setVMFirstBootDeviceFn func(ctx context.Context, name string, device client.BootDevice) error

	createHardDriveFn      func(ctx context.Context, opts client.HardDriveOptions) (*client.HardDrive, error)
	listHardDrivesFn       func(ctx context.Context, vmName string) ([]client.HardDrive, error)
	createDVDDriveFn       func(ctx context.Context, opts client.DVDDriveOptions) (*client.DVDDrive, error)
	listDVDDrivesFn        func(ctx context.Context, vmName string) ([]client.DVDDrive, error)
	createNetworkAdapterFn func(ctx context.Context, opts client.AdapterOptions) (*client.NetworkAdapter, error)
	listNetworkAdaptersFn  func(ctx context.Context, vmName string) ([]client.NetworkAdapter, error)
	deleteHardDriveFn      func(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int) error
	deleteDVDDriveFn       func(ctx context.Context, vmName string, controllerNumber, controllerLocation int) error
	deleteNetworkAdapterFn func(ctx context.Context, vmName, name string) error

	deleteVMCalled bool
}

func (m *mockHyperVClient) CreateVM(ctx context.Context, opts client.VMOptions) (*client.VM, error) {
	return m.createVMFn(ctx, opts)
}
func (m *mockHyperVClient) GetVM(ctx context.Context, name string) (*client.VM, error) {
	return m.getVMFn(ctx, name)
}
func (m *mockHyperVClient) DeleteVM(ctx context.Context, name string) error {
	m.deleteVMCalled = true
	if m.deleteVMFn != nil {
		return m.deleteVMFn(ctx, name)
	}
	return nil
}
func (m *mockHyperVClient) SetVMState(ctx context.Context, name string, state client.VMState) error {
	return m.setVMStateFn(ctx, name, state)
}
func (m *mockHyperVClient) SetVMFirmware(ctx context.Context, name string, opts client.VMFirmwareOptions) error {
	if m.setVMFirmwareFn != nil {
		return m.setVMFirmwareFn(ctx, name, opts)
	}
	return nil
}
func (m *mockHyperVClient) GetVMFirmware(ctx context.Context, name string) (*client.VMFirmware, error) {
	return m.getVMFirmwareFn(ctx, name)
}
func (m *mockHyperVClient) SetVMFirstBootDevice(ctx context.Context, name string, device client.BootDevice) error {
	return m.setVMFirstBootDeviceFn(ctx, name, device)
}
func (m *mockHyperVClient) CreateHardDrive(ctx context.Context, opts client.HardDriveOptions) (*client.HardDrive, error) {
	return m.createHardDriveFn(ctx, opts)
}
func (m *mockHyperVClient) ListHardDrives(ctx context.Context, vmName string) ([]client.HardDrive, error) {
	if m.listHardDrivesFn != nil {
		return m.listHardDrivesFn(ctx, vmName)
	}
	return nil, nil
}
func (m *mockHyperVClient) CreateDVDDrive(ctx context.Context, opts client.DVDDriveOptions) (*client.DVDDrive, error) {
	return m.createDVDDriveFn(ctx, opts)
}
func (m *mockHyperVClient) ListDVDDrives(ctx context.Context, vmName string) ([]client.DVDDrive, error) {
	if m.listDVDDrivesFn != nil {
		return m.listDVDDrivesFn(ctx, vmName)
	}
	return nil, nil
}
func (m *mockHyperVClient) CreateNetworkAdapter(ctx context.Context, opts client.AdapterOptions) (*client.NetworkAdapter, error) {
	return m.createNetworkAdapterFn(ctx, opts)
}
func (m *mockHyperVClient) ListNetworkAdapters(ctx context.Context, vmName string) ([]client.NetworkAdapter, error) {
	if m.listNetworkAdaptersFn != nil {
		return m.listNetworkAdaptersFn(ctx, vmName)
	}
	return nil, nil
}
func (m *mockHyperVClient) DeleteHardDrive(ctx context.Context, vmName, controllerType string, controllerNumber, controllerLocation int) error {
	if m.deleteHardDriveFn != nil {
		return m.deleteHardDriveFn(ctx, vmName, controllerType, controllerNumber, controllerLocation)
	}
	return nil
}
func (m *mockHyperVClient) DeleteDVDDrive(ctx context.Context, vmName string, controllerNumber, controllerLocation int) error {
	if m.deleteDVDDriveFn != nil {
		return m.deleteDVDDriveFn(ctx, vmName, controllerNumber, controllerLocation)
	}
	return nil
}
func (m *mockHyperVClient) DeleteNetworkAdapter(ctx context.Context, vmName, name string) error {
	if m.deleteNetworkAdapterFn != nil {
		return m.deleteNetworkAdapterFn(ctx, vmName, name)
	}
	return nil
}

// tftypes for constructing plan values
var firstBootDeviceTFType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"device_type":         tftypes.String,
		"controller_number":   tftypes.Number,
		"controller_location": tftypes.Number,
	},
}

var hardDriveTFType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"path":                tftypes.String,
		"controller_type":     tftypes.String,
		"controller_number":   tftypes.Number,
		"controller_location": tftypes.Number,
	},
}

var dvdDriveTFType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"path":                tftypes.String,
		"controller_number":   tftypes.Number,
		"controller_location": tftypes.Number,
	},
}

var networkAdapterTFType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"name":                tftypes.String,
		"switch_name":         tftypes.String,
		"vlan_id":             tftypes.Number,
		"mac_address":         tftypes.String,
		"dynamic_mac_address": tftypes.Bool,
	},
}

var vmTFType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"name":                   tftypes.String,
		"generation":             tftypes.Number,
		"processor_count":        tftypes.Number,
		"memory_startup_bytes":   tftypes.Number,
		"memory_minimum_bytes":   tftypes.Number,
		"memory_maximum_bytes":   tftypes.Number,
		"dynamic_memory":         tftypes.Bool,
		"state":                  tftypes.String,
		"notes":                  tftypes.String,
		"automatic_start_action": tftypes.String,
		"automatic_stop_action":  tftypes.String,
		"checkpoint_type":        tftypes.String,
		"secure_boot_enabled":    tftypes.Bool,
		"secure_boot_template":   tftypes.String,
		"first_boot_device":      firstBootDeviceTFType,
		"hard_drive":             tftypes.List{ElementType: hardDriveTFType},
		"dvd_drive":              tftypes.List{ElementType: dvdDriveTFType},
		"network_adapter":        tftypes.List{ElementType: networkAdapterTFType},
	},
}

type inlineBlocks struct {
	hardDrives      []tftypes.Value
	dvdDrives       []tftypes.Value
	networkAdapters []tftypes.Value
}

func buildPlanValue(state string, fbd *tftypes.Value) tftypes.Value {
	return buildPlanValueWithInline(state, fbd, nil)
}

func buildPlanValueWithInline(state string, fbd *tftypes.Value, inline *inlineBlocks) tftypes.Value {
	fbdVal := tftypes.NewValue(firstBootDeviceTFType, nil) // null
	if fbd != nil {
		fbdVal = *fbd
	}

	hdVal := tftypes.NewValue(tftypes.List{ElementType: hardDriveTFType}, nil) // null
	dvdVal := tftypes.NewValue(tftypes.List{ElementType: dvdDriveTFType}, nil)
	naVal := tftypes.NewValue(tftypes.List{ElementType: networkAdapterTFType}, nil)

	if inline != nil {
		if inline.hardDrives != nil {
			hdVal = tftypes.NewValue(tftypes.List{ElementType: hardDriveTFType}, inline.hardDrives)
		}
		if inline.dvdDrives != nil {
			dvdVal = tftypes.NewValue(tftypes.List{ElementType: dvdDriveTFType}, inline.dvdDrives)
		}
		if inline.networkAdapters != nil {
			naVal = tftypes.NewValue(tftypes.List{ElementType: networkAdapterTFType}, inline.networkAdapters)
		}
	}

	return tftypes.NewValue(vmTFType, map[string]tftypes.Value{
		"name":                   tftypes.NewValue(tftypes.String, "test-vm"),
		"generation":             tftypes.NewValue(tftypes.Number, int64(2)),
		"processor_count":        tftypes.NewValue(tftypes.Number, int64(1)),
		"memory_startup_bytes":   tftypes.NewValue(tftypes.Number, int64(536870912)),
		"memory_minimum_bytes":   tftypes.NewValue(tftypes.Number, int64(536870912)),
		"memory_maximum_bytes":   tftypes.NewValue(tftypes.Number, int64(1073741824)),
		"dynamic_memory":         tftypes.NewValue(tftypes.Bool, false),
		"state":                  tftypes.NewValue(tftypes.String, state),
		"notes":                  tftypes.NewValue(tftypes.String, ""),
		"automatic_start_action": tftypes.NewValue(tftypes.String, "Nothing"),
		"automatic_stop_action":  tftypes.NewValue(tftypes.String, "TurnOff"),
		"checkpoint_type":        tftypes.NewValue(tftypes.String, "Disabled"),
		"secure_boot_enabled":    tftypes.NewValue(tftypes.Bool, nil),
		"secure_boot_template":   tftypes.NewValue(tftypes.String, nil),
		"first_boot_device":      fbdVal,
		"hard_drive":             hdVal,
		"dvd_drive":              dvdVal,
		"network_adapter":        naVal,
	})
}

func defaultVM(state string) *client.VM {
	return &client.VM{
		Name:                 "test-vm",
		Generation:           2,
		ProcessorCount:       1,
		MemoryStartup:        536870912,
		MemoryMinimum:        536870912,
		MemoryMaximum:        1073741824,
		DynamicMemoryEnabled: false,
		State:                state,
		Notes:                "",
		AutomaticStartAction: "Nothing",
		AutomaticStopAction:  "TurnOff",
		CheckpointType:       "Disabled",
	}
}

func defaultFirmware() *client.VMFirmware {
	return &client.VMFirmware{
		SecureBootEnabled:  "On",
		SecureBootTemplate: "MicrosoftWindows",
	}
}

func callVMCreate(t *testing.T, mock *mockHyperVClient, planValue tftypes.Value) resource.CreateResponse {
	t.Helper()

	r := &vmResource{client: mock}

	var schResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schResp)
	sch := schResp.Schema

	req := resource.CreateRequest{
		Plan: tfsdk.Plan{
			Schema: sch,
			Raw:    planValue,
		},
	}

	var resp resource.CreateResponse
	resp.State = tfsdk.State{Schema: sch}

	r.Create(context.Background(), req, &resp)
	return resp
}

func hasWarning(diags diag.Diagnostics, substr string) bool {
	for _, d := range diags {
		if d.Severity() == diag.SeverityWarning && strings.Contains(d.Summary(), substr) {
			return true
		}
	}
	return false
}

func hasError(diags diag.Diagnostics) bool {
	return diags.HasError()
}

func TestVMCreate_FirstBootDeviceNotAttached_Warning(t *testing.T) {
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
		setVMFirmwareFn: func(_ context.Context, _ string, _ client.VMFirmwareOptions) error {
			return fmt.Errorf("the drive at controller 0:0 is not attached yet")
		},
	}

	fbd := tftypes.NewValue(firstBootDeviceTFType, map[string]tftypes.Value{
		"device_type":         tftypes.NewValue(tftypes.String, "HardDiskDrive"),
		"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
		"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
	})

	resp := callVMCreate(t, mock, buildPlanValue("Off", &fbd))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if !hasWarning(resp.Diagnostics, "not attached yet") {
		t.Errorf("expected warning about 'not attached yet', got: %v", resp.Diagnostics)
	}
	if mock.deleteVMCalled {
		t.Error("DeleteVM should NOT be called for 'not attached yet' error")
	}

	// Issue #7: first_boot_device must be preserved in state (not null)
	// so Terraform doesn't report "inconsistent result after apply".
	var state vmResourceModel
	resp.State.Get(context.Background(), &state)
	if state.FirstBootDevice.IsNull() {
		t.Error("first_boot_device should be preserved in state, got null")
	}
}

func TestVMCreate_FirstBootDeviceOtherError_DeletesVM(t *testing.T) {
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		setVMFirmwareFn: func(_ context.Context, _ string, _ client.VMFirmwareOptions) error {
			return fmt.Errorf("unexpected firmware error")
		},
	}

	fbd := tftypes.NewValue(firstBootDeviceTFType, map[string]tftypes.Value{
		"device_type":         tftypes.NewValue(tftypes.String, "HardDiskDrive"),
		"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
		"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
	})

	resp := callVMCreate(t, mock, buildPlanValue("Off", &fbd))

	if !hasError(resp.Diagnostics) {
		t.Fatal("expected an error diagnostic")
	}
	if !mock.deleteVMCalled {
		t.Error("DeleteVM should be called on non-'not attached yet' error")
	}
}

func TestVMCreate_StateRunning_Warning(t *testing.T) {
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		setVMStateFn: func(_ context.Context, _ string, _ client.VMState) error {
			return nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Running"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
	}

	resp := callVMCreate(t, mock, buildPlanValue("Running", nil))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if !hasWarning(resp.Diagnostics, "VM starting before drives may be attached") {
		t.Errorf("expected warning about starting before drives attached, got: %v", resp.Diagnostics)
	}
}

func TestVMCreate_StateOff_NoWarning(t *testing.T) {
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
	}

	resp := callVMCreate(t, mock, buildPlanValue("Off", nil))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if hasWarning(resp.Diagnostics, "drives") {
		t.Errorf("expected no warnings about drives, got: %v", resp.Diagnostics)
	}
	if hasWarning(resp.Diagnostics, "not attached") {
		t.Errorf("expected no warnings about boot device, got: %v", resp.Diagnostics)
	}
}

func TestVMCreate_InlineHardDrive(t *testing.T) {
	createHardDriveCalled := false
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
		setVMFirmwareFn: func(_ context.Context, _ string, _ client.VMFirmwareOptions) error {
			return nil
		},
		createHardDriveFn: func(_ context.Context, opts client.HardDriveOptions) (*client.HardDrive, error) {
			createHardDriveCalled = true
			return &client.HardDrive{
				VMName:             opts.VMName,
				Path:               opts.Path,
				ControllerType:     1, // SCSI
				ControllerNumber:   opts.ControllerNumber,
				ControllerLocation: opts.ControllerLocation,
			}, nil
		},
	}

	fbd := tftypes.NewValue(firstBootDeviceTFType, map[string]tftypes.Value{
		"device_type":         tftypes.NewValue(tftypes.String, "HardDiskDrive"),
		"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
		"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
	})

	inline := &inlineBlocks{
		hardDrives: []tftypes.Value{
			tftypes.NewValue(hardDriveTFType, map[string]tftypes.Value{
				"path":                tftypes.NewValue(tftypes.String, "C:\\VMs\\disk.vhdx"),
				"controller_type":     tftypes.NewValue(tftypes.String, "SCSI"),
				"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
				"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
			}),
		},
	}

	resp := callVMCreate(t, mock, buildPlanValueWithInline("Off", &fbd, inline))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if !createHardDriveCalled {
		t.Error("CreateHardDrive should have been called")
	}
	// Boot device set should succeed since drive is inline (no warning)
	if hasWarning(resp.Diagnostics, "not attached yet") {
		t.Error("should not have 'not attached yet' warning when drive is inline")
	}
}

func TestVMCreate_InlineHardDriveError_RollsBack(t *testing.T) {
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		createHardDriveFn: func(_ context.Context, _ client.HardDriveOptions) (*client.HardDrive, error) {
			return nil, fmt.Errorf("disk not found")
		},
	}

	inline := &inlineBlocks{
		hardDrives: []tftypes.Value{
			tftypes.NewValue(hardDriveTFType, map[string]tftypes.Value{
				"path":                tftypes.NewValue(tftypes.String, "C:\\bad\\path.vhdx"),
				"controller_type":     tftypes.NewValue(tftypes.String, "SCSI"),
				"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
				"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
			}),
		},
	}

	resp := callVMCreate(t, mock, buildPlanValueWithInline("Off", nil, inline))

	if !hasError(resp.Diagnostics) {
		t.Fatal("expected an error diagnostic")
	}
	if !mock.deleteVMCalled {
		t.Error("DeleteVM should be called when CreateHardDrive fails (rollback)")
	}
}

func TestVMCreate_InlineAllTypes(t *testing.T) {
	createHDCalled := false
	createDVDCalled := false
	createNICCalled := false

	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
		createHardDriveFn: func(_ context.Context, opts client.HardDriveOptions) (*client.HardDrive, error) {
			createHDCalled = true
			return &client.HardDrive{
				VMName: opts.VMName, Path: opts.Path, ControllerType: 1,
				ControllerNumber: opts.ControllerNumber, ControllerLocation: opts.ControllerLocation,
			}, nil
		},
		createDVDDriveFn: func(_ context.Context, opts client.DVDDriveOptions) (*client.DVDDrive, error) {
			createDVDCalled = true
			return &client.DVDDrive{
				VMName: opts.VMName, Path: opts.Path,
				ControllerNumber: opts.ControllerNumber, ControllerLocation: opts.ControllerLocation,
			}, nil
		},
		createNetworkAdapterFn: func(_ context.Context, opts client.AdapterOptions) (*client.NetworkAdapter, error) {
			createNICCalled = true
			return &client.NetworkAdapter{
				Name: opts.Name, VMName: opts.VMName, SwitchName: opts.SwitchName,
				MacAddress: "000000000000", DynamicMacAddress: true,
			}, nil
		},
	}

	inline := &inlineBlocks{
		hardDrives: []tftypes.Value{
			tftypes.NewValue(hardDriveTFType, map[string]tftypes.Value{
				"path":                tftypes.NewValue(tftypes.String, "C:\\VMs\\disk.vhdx"),
				"controller_type":     tftypes.NewValue(tftypes.String, "SCSI"),
				"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
				"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
			}),
		},
		dvdDrives: []tftypes.Value{
			tftypes.NewValue(dvdDriveTFType, map[string]tftypes.Value{
				"path":                tftypes.NewValue(tftypes.String, "C:\\ISOs\\boot.iso"),
				"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
				"controller_location": tftypes.NewValue(tftypes.Number, int64(1)),
			}),
		},
		networkAdapters: []tftypes.Value{
			tftypes.NewValue(networkAdapterTFType, map[string]tftypes.Value{
				"name":                tftypes.NewValue(tftypes.String, "Primary"),
				"switch_name":         tftypes.NewValue(tftypes.String, "Default Switch"),
				"vlan_id":             tftypes.NewValue(tftypes.Number, nil),
				"mac_address":         tftypes.NewValue(tftypes.String, nil),
				"dynamic_mac_address": tftypes.NewValue(tftypes.Bool, true),
			}),
		},
	}

	resp := callVMCreate(t, mock, buildPlanValueWithInline("Off", nil, inline))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if !createHDCalled {
		t.Error("CreateHardDrive should have been called")
	}
	if !createDVDCalled {
		t.Error("CreateDVDDrive should have been called")
	}
	if !createNICCalled {
		t.Error("CreateNetworkAdapter should have been called")
	}
}

func TestVMCreate_InlineWithRunning_NoWarning(t *testing.T) {
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		setVMStateFn: func(_ context.Context, _ string, _ client.VMState) error {
			return nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Running"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
		createHardDriveFn: func(_ context.Context, opts client.HardDriveOptions) (*client.HardDrive, error) {
			return &client.HardDrive{
				VMName: opts.VMName, Path: opts.Path, ControllerType: 1,
				ControllerNumber: opts.ControllerNumber, ControllerLocation: opts.ControllerLocation,
			}, nil
		},
	}

	inline := &inlineBlocks{
		hardDrives: []tftypes.Value{
			tftypes.NewValue(hardDriveTFType, map[string]tftypes.Value{
				"path":                tftypes.NewValue(tftypes.String, "C:\\VMs\\disk.vhdx"),
				"controller_type":     tftypes.NewValue(tftypes.String, "SCSI"),
				"controller_number":   tftypes.NewValue(tftypes.Number, int64(0)),
				"controller_location": tftypes.NewValue(tftypes.Number, int64(0)),
			}),
		},
	}

	resp := callVMCreate(t, mock, buildPlanValueWithInline("Running", nil, inline))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	// When inline blocks are present, the "drives may not be attached" warning should be suppressed
	if hasWarning(resp.Diagnostics, "VM starting before drives may be attached") {
		t.Error("should not warn about drives when inline blocks are present")
	}
}

func TestVMCreate_NoInlineBlocks_Unchanged(t *testing.T) {
	// Existing behavior: no inline blocks, no inline mock methods needed
	mock := &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFn: func(_ context.Context, _ string) (*client.VM, error) {
			return defaultVM("Off"), nil
		},
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return defaultFirmware(), nil
		},
	}

	resp := callVMCreate(t, mock, buildPlanValue("Off", nil))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}

	var state vmResourceModel
	resp.State.Get(context.Background(), &state)

	// Inline lists should be null when not configured
	if !state.HardDrives.IsNull() {
		t.Error("hard_drive should be null when not configured")
	}
	if !state.DVDDrives.IsNull() {
		t.Error("dvd_drive should be null when not configured")
	}
	if !state.NetworkAdapters.IsNull() {
		t.Error("network_adapter should be null when not configured")
	}
}
