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
	return m.setVMFirmwareFn(ctx, name, opts)
}
func (m *mockHyperVClient) GetVMFirmware(ctx context.Context, name string) (*client.VMFirmware, error) {
	return m.getVMFirmwareFn(ctx, name)
}
func (m *mockHyperVClient) SetVMFirstBootDevice(ctx context.Context, name string, device client.BootDevice) error {
	return m.setVMFirstBootDeviceFn(ctx, name, device)
}

// tftypes for constructing plan values
var firstBootDeviceTFType = tftypes.Object{
	AttributeTypes: map[string]tftypes.Type{
		"device_type":         tftypes.String,
		"controller_number":   tftypes.Number,
		"controller_location": tftypes.Number,
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
	},
}

func buildPlanValue(state string, fbd *tftypes.Value) tftypes.Value {
	fbdVal := tftypes.NewValue(firstBootDeviceTFType, nil) // null
	if fbd != nil {
		fbdVal = *fbd
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
		setVMFirstBootDeviceFn: func(_ context.Context, _ string, _ client.BootDevice) error {
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
		setVMFirstBootDeviceFn: func(_ context.Context, _ string, _ client.BootDevice) error {
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
