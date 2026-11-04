package resources

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

// gen2PlanAllComputedUnknown is the plan Terraform sends on create for a Gen2
// VM whose config leaves first_boot_device and the secure-boot attributes
// unset: Optional+Computed attributes with no prior state plan as unknown.
func gen2PlanAllComputedUnknown() tftypes.Value {
	v := gen1Value(tftypes.UnknownValue, tftypes.UnknownValue)
	var attrs map[string]tftypes.Value
	_ = v.As(&attrs)
	attrs["generation"] = tftypes.NewValue(tftypes.Number, int64(2))
	attrs["first_boot_device"] = tftypes.NewValue(firstBootDeviceTFType, tftypes.UnknownValue)
	return tftypes.NewValue(vmTFType, attrs)
}

func gen2CreateMock(fw *client.VMFirmware, fwErr error) *mockHyperVClient {
	return &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) { return defaultVM("Off"), nil },
		getVMFn:    func(_ context.Context, _ string) (*client.VM, error) { return defaultVM("Off"), nil },
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			return fw, fwErr
		},
	}
}

// Issue #20: unset first_boot_device on Gen2 left the attribute unknown after apply.
func TestVMCreate_Gen2_UnsetFirstBootDevice_StateFullyKnown(t *testing.T) {
	mock := gen2CreateMock(&client.VMFirmware{SecureBootEnabled: "On", SecureBootTemplate: "MicrosoftWindows"}, nil)

	resp := callVMCreate(t, mock, gen2PlanAllComputedUnknown())

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsFullyKnown() {
		t.Fatalf("state must be fully known after apply, got: %v", resp.State.Raw)
	}
	var state vmResourceModel
	resp.State.Get(context.Background(), &state)
	if !state.FirstBootDevice.IsNull() {
		t.Errorf("first_boot_device: want null when host reports none, got %v", state.FirstBootDevice)
	}
	if !state.SecureBootEnabled.ValueBool() || state.SecureBootTemplate.ValueString() != "MicrosoftWindows" {
		t.Errorf("secure boot read-back not applied: enabled=%v template=%v", state.SecureBootEnabled, state.SecureBootTemplate)
	}
}

// Issue #20, second gap: a failed Get-VMFirmware must not leave unknowns in state.
func TestVMCreate_Gen2_FirmwareReadError_StateFullyKnown(t *testing.T) {
	mock := gen2CreateMock(nil, errors.New("transient WinRM failure"))

	resp := callVMCreate(t, mock, gen2PlanAllComputedUnknown())

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected only a warning, got errors: %v", resp.Diagnostics)
	}
	if !hasWarning(resp.Diagnostics, "Error reading VM firmware") {
		t.Errorf("expected firmware read warning, got: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsFullyKnown() {
		t.Fatalf("state must be fully known after apply, got: %v", resp.State.Raw)
	}
}
