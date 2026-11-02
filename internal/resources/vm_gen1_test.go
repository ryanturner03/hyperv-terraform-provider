package resources

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

// gen1Value builds a full hyperv_vm object value for a Generation 1 VM with
// the secure-boot attributes set to the given values. Pass
// tftypes.UnknownValue to mimic what Terraform plans for an Optional+Computed
// attribute that is absent from config.
func gen1Value(secureBoot, template interface{}) tftypes.Value {
	return tftypes.NewValue(vmTFType, map[string]tftypes.Value{
		"name":                   tftypes.NewValue(tftypes.String, "test-vm"),
		"generation":             tftypes.NewValue(tftypes.Number, int64(1)),
		"processor_count":        tftypes.NewValue(tftypes.Number, int64(1)),
		"memory_startup_bytes":   tftypes.NewValue(tftypes.Number, int64(536870912)),
		"memory_minimum_bytes":   tftypes.NewValue(tftypes.Number, int64(536870912)),
		"memory_maximum_bytes":   tftypes.NewValue(tftypes.Number, int64(1073741824)),
		"dynamic_memory":         tftypes.NewValue(tftypes.Bool, false),
		"state":                  tftypes.NewValue(tftypes.String, "Off"),
		"notes":                  tftypes.NewValue(tftypes.String, ""),
		"automatic_start_action": tftypes.NewValue(tftypes.String, "Nothing"),
		"automatic_stop_action":  tftypes.NewValue(tftypes.String, "TurnOff"),
		"checkpoint_type":        tftypes.NewValue(tftypes.String, "Disabled"),
		"secure_boot_enabled":    tftypes.NewValue(tftypes.Bool, secureBoot),
		"secure_boot_template":   tftypes.NewValue(tftypes.String, template),
		"first_boot_device":      tftypes.NewValue(firstBootDeviceTFType, nil),
		"hard_drive":             tftypes.NewValue(tftypes.List{ElementType: hardDriveTFType}, nil),
		"dvd_drive":              tftypes.NewValue(tftypes.List{ElementType: dvdDriveTFType}, nil),
		"network_adapter":        tftypes.NewValue(tftypes.List{ElementType: networkAdapterTFType}, nil),
	})
}

func gen1VM() *client.VM {
	vm := defaultVM("Off")
	vm.Generation = 1
	return vm
}

// noFirmwareMock fails the test if any firmware cmdlet is invoked; those
// error on Gen1 hosts.
func noFirmwareMock(t *testing.T) *mockHyperVClient {
	t.Helper()
	return &mockHyperVClient{
		createVMFn: func(_ context.Context, _ client.VMOptions) (*client.VM, error) { return gen1VM(), nil },
		getVMFn:    func(_ context.Context, _ string) (*client.VM, error) { return gen1VM(), nil },
		getVMFirmwareFn: func(_ context.Context, _ string) (*client.VMFirmware, error) {
			t.Error("Get-VMFirmware must not be called on a Generation 1 VM")
			return nil, nil
		},
		setVMFirmwareFn: func(_ context.Context, _ string, _ client.VMFirmwareOptions) error {
			t.Error("Set-VMFirmware must not be called on a Generation 1 VM")
			return nil
		},
	}
}

func assertGen1SecureBootState(t *testing.T, m vmResourceModel) {
	t.Helper()
	if m.SecureBootEnabled.IsUnknown() || m.SecureBootEnabled.IsNull() || m.SecureBootEnabled.ValueBool() {
		t.Errorf("secure_boot_enabled: want known false, got %v", m.SecureBootEnabled)
	}
	if m.SecureBootTemplate.IsUnknown() || m.SecureBootTemplate.IsNull() || m.SecureBootTemplate.ValueString() != "" {
		t.Errorf("secure_boot_template: want known \"\", got %v", m.SecureBootTemplate)
	}
	if !m.FirstBootDevice.IsNull() {
		t.Errorf("first_boot_device: want null, got %v", m.FirstBootDevice)
	}
}

func TestVMCreate_Gen1_SecureBootUnset_ProducesKnownState(t *testing.T) {
	mock := noFirmwareMock(t)

	resp := callVMCreate(t, mock, gen1Value(tftypes.UnknownValue, tftypes.UnknownValue))

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsFullyKnown() {
		t.Fatalf("state must be fully known after apply, got: %v", resp.State.Raw)
	}
	var state vmResourceModel
	resp.State.Get(context.Background(), &state)
	assertGen1SecureBootState(t, state)
}

func TestVMModifyPlan_Gen1_SecureBootPlannedKnown(t *testing.T) {
	r := &vmResource{}
	var schResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schResp)
	sch := schResp.Schema

	planVal := gen1Value(tftypes.UnknownValue, tftypes.UnknownValue)
	req := resource.ModifyPlanRequest{
		Config: tfsdk.Config{Schema: sch, Raw: gen1Value(nil, nil)},
		Plan:   tfsdk.Plan{Schema: sch, Raw: planVal},
		State:  tfsdk.State{Schema: sch, Raw: tftypes.NewValue(vmTFType, nil)},
	}
	resp := resource.ModifyPlanResponse{Plan: tfsdk.Plan{Schema: sch, Raw: planVal}}

	r.ModifyPlan(context.Background(), req, &resp)

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	var plan vmResourceModel
	resp.Plan.Get(context.Background(), &plan)
	assertGen1SecureBootState(t, plan)
}

func TestVMRead_Gen1_SecureBootFixedWithoutFirmwareQuery(t *testing.T) {
	mock := noFirmwareMock(t)
	r := &vmResource{client: mock}
	var schResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schResp)
	sch := schResp.Schema

	req := resource.ReadRequest{State: tfsdk.State{Schema: sch, Raw: gen1Value(false, "")}}
	resp := resource.ReadResponse{State: tfsdk.State{Schema: sch, Raw: gen1Value(false, "")}}

	r.Read(context.Background(), req, &resp)

	if hasError(resp.Diagnostics) {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
	var state vmResourceModel
	resp.State.Get(context.Background(), &state)
	assertGen1SecureBootState(t, state)
}

func callValidateConfig(t *testing.T, cfg tftypes.Value) resource.ValidateConfigResponse {
	t.Helper()
	r := &vmResource{}
	var schResp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &schResp)
	var resp resource.ValidateConfigResponse
	r.ValidateConfig(context.Background(), resource.ValidateConfigRequest{
		Config: tfsdk.Config{Schema: schResp.Schema, Raw: cfg},
	}, &resp)
	return resp
}

func TestVMValidateConfig_Gen1_SecureBootFalseAccepted(t *testing.T) {
	resp := callValidateConfig(t, gen1Value(false, nil))
	if hasError(resp.Diagnostics) {
		t.Fatalf("secure_boot_enabled = false must be accepted on Gen1, got: %v", resp.Diagnostics)
	}
}

func TestVMValidateConfig_Gen1_SecureBootTrueRejected(t *testing.T) {
	resp := callValidateConfig(t, gen1Value(true, nil))
	if !hasError(resp.Diagnostics) {
		t.Fatal("secure_boot_enabled = true must be rejected on Gen1")
	}
}

func TestVMValidateConfig_Gen1_SecureBootTemplateRejected(t *testing.T) {
	resp := callValidateConfig(t, gen1Value(nil, "MicrosoftWindows"))
	if !hasError(resp.Diagnostics) {
		t.Fatal("secure_boot_template must be rejected on Gen1")
	}
}
