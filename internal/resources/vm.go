package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource                = &vmResource{}
	_ resource.ResourceWithConfigure   = &vmResource{}
	_ resource.ResourceWithValidateConfig = &vmResource{}
)

type vmResource struct {
	client client.HyperVClient
}

type firstBootDeviceModel struct {
	DeviceType         types.String `tfsdk:"device_type"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
}

type vmResourceModel struct {
	Name                 types.String `tfsdk:"name"`
	Generation           types.Int64  `tfsdk:"generation"`
	ProcessorCount       types.Int64  `tfsdk:"processor_count"`
	MemoryStartupBytes   types.Int64  `tfsdk:"memory_startup_bytes"`
	MemoryMinimumBytes   types.Int64  `tfsdk:"memory_minimum_bytes"`
	MemoryMaximumBytes   types.Int64  `tfsdk:"memory_maximum_bytes"`
	DynamicMemory        types.Bool   `tfsdk:"dynamic_memory"`
	State                types.String `tfsdk:"state"`
	Notes                types.String `tfsdk:"notes"`
	AutomaticStartAction types.String `tfsdk:"automatic_start_action"`
	AutomaticStopAction  types.String `tfsdk:"automatic_stop_action"`
	CheckpointType       types.String `tfsdk:"checkpoint_type"`
	SecureBootEnabled    types.Bool   `tfsdk:"secure_boot_enabled"`
	SecureBootTemplate   types.String `tfsdk:"secure_boot_template"`
	FirstBootDevice      types.Object `tfsdk:"first_boot_device"`
}

func NewVMResource() resource.Resource {
	return &vmResource{}
}

func (r *vmResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (r *vmResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Hyper-V virtual machine.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"generation": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(2),
				Description: "The generation of the virtual machine (1 or 2). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"processor_count": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "The number of virtual processors assigned to the VM.",
			},
			"memory_startup_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(536870912), // 512 MB
				Description: "The startup memory in bytes. Default: 512MB.",
			},
			"memory_minimum_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The minimum memory in bytes when dynamic memory is enabled.",
			},
			"memory_maximum_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The maximum memory in bytes when dynamic memory is enabled.",
			},
			"dynamic_memory": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether dynamic memory is enabled.",
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Off"),
				Description: "The desired state of the VM: Running or Off.",
				Validators: []validator.String{
					stringvalidator.OneOf("Running", "Off"),
				},
			},
			"notes": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Notes or description for the virtual machine.",
			},
			"automatic_start_action": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The action to take when the host starts (Nothing, Start, StartIfRunning).",
				Validators: []validator.String{
					stringvalidator.OneOf("Nothing", "StartIfRunning", "Start"),
				},
			},
			"automatic_stop_action": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The action to take when the host shuts down (TurnOff, Save, ShutDown).",
				Validators: []validator.String{
					stringvalidator.OneOf("TurnOff", "Save", "ShutDown"),
				},
			},
			"checkpoint_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The checkpoint type (Disabled, Production, ProductionOnly, Standard).",
				Validators: []validator.String{
					stringvalidator.OneOf("Disabled", "Production", "ProductionOnly", "Standard"),
				},
			},
			"secure_boot_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether Secure Boot is enabled (Generation 2 only).",
			},
			"secure_boot_template": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The Secure Boot template (Generation 2 only). Values: MicrosoftWindows, MicrosoftUEFICertificateAuthority.",
				Validators: []validator.String{
					stringvalidator.OneOf("MicrosoftWindows", "MicrosoftUEFICertificateAuthority"),
				},
			},
			"first_boot_device": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The first boot device (Generation 2 only). Note: the referenced drive must already exist on the VM.",
				Attributes: map[string]schema.Attribute{
					"device_type": schema.StringAttribute{
						Required:    true,
						Description: "The type of boot device: HardDiskDrive, DvdDrive, or NetworkAdapter.",
						Validators: []validator.String{
							stringvalidator.OneOf("HardDiskDrive", "DvdDrive", "NetworkAdapter"),
						},
					},
					"controller_number": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(0),
						Description: "The controller number (default: 0).",
					},
					"controller_location": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Default:     int64default.StaticInt64(0),
						Description: "The controller location (default: 0).",
					},
				},
			},
		},
	}
}

func (r *vmResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config vmResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generation may be unknown during validation (e.g. from a variable), so only
	// validate when we have a concrete value.
	if config.Generation.IsNull() || config.Generation.IsUnknown() {
		return
	}

	gen := config.Generation.ValueInt64()
	if gen == 1 {
		if !config.SecureBootEnabled.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"secure_boot_enabled is only supported on Generation 2 VMs.",
			)
		}
		if !config.SecureBootTemplate.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"secure_boot_template is only supported on Generation 2 VMs.",
			)
		}
		if !config.FirstBootDevice.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid Configuration",
				"first_boot_device is only supported on Generation 2 VMs.",
			)
		}
	}
}

func (r *vmResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(client.HyperVClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected client.HyperVClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = c
}

func (r *vmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.VMOptions{
		Name:                 plan.Name.ValueString(),
		Generation:           int(plan.Generation.ValueInt64()),
		ProcessorCount:       int(plan.ProcessorCount.ValueInt64()),
		MemoryStartupBytes:   plan.MemoryStartupBytes.ValueInt64(),
		MemoryMinimumBytes:   plan.MemoryMinimumBytes.ValueInt64(),
		MemoryMaximumBytes:   plan.MemoryMaximumBytes.ValueInt64(),
		DynamicMemory:        plan.DynamicMemory.ValueBool(),
		Notes:                plan.Notes.ValueString(),
		AutomaticStartAction: plan.AutomaticStartAction.ValueString(),
		AutomaticStopAction:  plan.AutomaticStopAction.ValueString(),
		CheckpointType:       plan.CheckpointType.ValueString(),
	}

	vm, err := r.client.CreateVM(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating VM", err.Error())
		return
	}

	vmName := vm.Name
	isGen2 := vm.Generation == 2

	// Configure firmware for Gen 2 VMs (secure boot + template, no drive dependency)
	if isGen2 {
		fwOpts := r.buildFirmwareOpts(plan)
		if fwOpts != nil {
			if err := r.client.SetVMFirmware(ctx, vmName, *fwOpts); err != nil {
				_ = r.client.DeleteVM(ctx, vmName)
				resp.Diagnostics.AddError("Error configuring VM firmware", err.Error())
				return
			}
		}
	}

	// Set first boot device if configured (Gen 2 only)
	if isGen2 && !plan.FirstBootDevice.IsNull() && !plan.FirstBootDevice.IsUnknown() {
		var fbd firstBootDeviceModel
		resp.Diagnostics.Append(plan.FirstBootDevice.As(ctx, &fbd, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		device := client.BootDevice{
			DeviceType:         fbd.DeviceType.ValueString(),
			ControllerNumber:   int(fbd.ControllerNumber.ValueInt64()),
			ControllerLocation: int(fbd.ControllerLocation.ValueInt64()),
		}
		if err := r.client.SetVMFirstBootDevice(ctx, vmName, device); err != nil {
			_ = r.client.DeleteVM(ctx, vmName)
			resp.Diagnostics.AddError(
				"Error setting first boot device",
				fmt.Sprintf("Drive at controller %d:%d not attached yet. Ensure drive resources are created first, or create the VM with state = \"Off\" and update to \"Running\" after drives exist. Detail: %s",
					device.ControllerNumber, device.ControllerLocation, err.Error()),
			)
			return
		}
	}

	// Start VM if requested
	if plan.State.ValueString() == "Running" {
		if err := r.client.SetVMState(ctx, vmName, client.VMStateRunning); err != nil {
			_ = r.client.DeleteVM(ctx, vmName)
			resp.Diagnostics.AddError("Error starting VM", err.Error())
			return
		}
	}

	// Re-read VM to capture final state (e.g., after starting)
	vm, err = r.client.GetVM(ctx, vmName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading VM after create", err.Error())
		return
	}

	mapVMToState(vm, &plan, &resp.Diagnostics)

	if isGen2 {
		r.readFirmwareState(ctx, vmName, &plan, &resp.Diagnostics)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vm, err := r.client.GetVM(ctx, state.Name.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "ObjectNotFound") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading VM", err.Error())
		return
	}

	mapVMToState(vm, &state, &resp.Diagnostics)

	if vm.Generation == 2 {
		r.readFirmwareState(ctx, vm.Name, &state, &resp.Diagnostics)
	} else {
		state.SecureBootEnabled = types.BoolNull()
		state.SecureBootTemplate = types.StringNull()
		state.FirstBootDevice = types.ObjectNull(firstBootDeviceAttrTypes())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vmResourceModel
	var state vmResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()

	opts := client.VMOptions{
		Name:                 plan.Name.ValueString(),
		Generation:           int(plan.Generation.ValueInt64()),
		ProcessorCount:       int(plan.ProcessorCount.ValueInt64()),
		MemoryStartupBytes:   plan.MemoryStartupBytes.ValueInt64(),
		MemoryMinimumBytes:   plan.MemoryMinimumBytes.ValueInt64(),
		MemoryMaximumBytes:   plan.MemoryMaximumBytes.ValueInt64(),
		DynamicMemory:        plan.DynamicMemory.ValueBool(),
		Notes:                plan.Notes.ValueString(),
		AutomaticStartAction: plan.AutomaticStartAction.ValueString(),
		AutomaticStopAction:  plan.AutomaticStopAction.ValueString(),
		CheckpointType:       plan.CheckpointType.ValueString(),
	}

	isGen2 := plan.Generation.ValueInt64() == 2
	plannedState := plan.State.ValueString()
	currentState := state.State.ValueString()

	// Firmware and first boot device changes require the VM to be off.
	// If the VM is running, stop it before making config changes, then
	// restart it afterward if the planned state is still "Running".
	needsFirmware := isGen2 && r.buildFirmwareOpts(plan) != nil
	needsBootDevice := isGen2 && !plan.FirstBootDevice.IsNull() && !plan.FirstBootDevice.IsUnknown()
	stoppedForConfig := false

	if currentState == "Running" && (needsFirmware || needsBootDevice) {
		if err := r.client.SetVMState(ctx, name, client.VMStateOff); err != nil {
			resp.Diagnostics.AddError("Error stopping VM for firmware changes", err.Error())
			return
		}
		stoppedForConfig = true
	}

	err := r.client.UpdateVM(ctx, name, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error updating VM", err.Error())
		return
	}

	// Configure firmware for Gen 2 VMs
	if needsFirmware {
		fwOpts := r.buildFirmwareOpts(plan)
		if err := r.client.SetVMFirmware(ctx, name, *fwOpts); err != nil {
			resp.Diagnostics.AddError("Error configuring VM firmware", err.Error())
			return
		}
	}

	// Set first boot device if configured (Gen 2 only)
	if needsBootDevice {
		var fbd firstBootDeviceModel
		resp.Diagnostics.Append(plan.FirstBootDevice.As(ctx, &fbd, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		device := client.BootDevice{
			DeviceType:         fbd.DeviceType.ValueString(),
			ControllerNumber:   int(fbd.ControllerNumber.ValueInt64()),
			ControllerLocation: int(fbd.ControllerLocation.ValueInt64()),
		}
		if err := r.client.SetVMFirstBootDevice(ctx, name, device); err != nil {
			resp.Diagnostics.AddError(
				"Error setting first boot device",
				fmt.Sprintf("Drive at controller %d:%d not attached yet. Ensure drive resources are created first, or create the VM with state = \"Off\" and update to \"Running\" after drives exist. Detail: %s",
					device.ControllerNumber, device.ControllerLocation, err.Error()),
			)
			return
		}
	}

	// Handle state change: start if planned Running, or restart if we stopped for config
	if plannedState != currentState || stoppedForConfig {
		targetState := client.VMState(plannedState)
		if err := r.client.SetVMState(ctx, name, targetState); err != nil {
			resp.Diagnostics.AddError("Error setting VM state", err.Error())
			return
		}
	}

	vm, err := r.client.GetVM(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Error reading VM after update", err.Error())
		return
	}

	mapVMToState(vm, &plan, &resp.Diagnostics)

	if isGen2 {
		r.readFirmwareState(ctx, name, &plan, &resp.Diagnostics)
	} else {
		plan.SecureBootEnabled = types.BoolNull()
		plan.SecureBootTemplate = types.StringNull()
		plan.FirstBootDevice = types.ObjectNull(firstBootDeviceAttrTypes())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVM(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting VM", err.Error())
		return
	}
}

// buildFirmwareOpts returns firmware options to set, or nil if nothing needs changing.
func (r *vmResource) buildFirmwareOpts(plan vmResourceModel) *client.VMFirmwareOptions {
	opts := client.VMFirmwareOptions{}
	changed := false

	if !plan.SecureBootEnabled.IsNull() && !plan.SecureBootEnabled.IsUnknown() {
		v := plan.SecureBootEnabled.ValueBool()
		opts.SecureBootEnabled = &v
		changed = true
	}
	if !plan.SecureBootTemplate.IsNull() && !plan.SecureBootTemplate.IsUnknown() {
		opts.SecureBootTemplate = plan.SecureBootTemplate.ValueString()
		changed = true
	}

	if !changed {
		return nil
	}
	return &opts
}

// readFirmwareState reads firmware info from the host and populates model fields.
func (r *vmResource) readFirmwareState(ctx context.Context, vmName string, model *vmResourceModel, diags *diag.Diagnostics) {
	fw, err := r.client.GetVMFirmware(ctx, vmName)
	if err != nil {
		diags.AddWarning("Error reading VM firmware", err.Error())
		return
	}

	model.SecureBootEnabled = types.BoolValue(fw.SecureBootEnabled == "On")
	model.SecureBootTemplate = types.StringValue(fw.SecureBootTemplate)

	if fw.FirstBootDeviceType != "" && fw.FirstBootDeviceType != "Unknown" {
		fbd, d := types.ObjectValueFrom(ctx, firstBootDeviceAttrTypes(), firstBootDeviceModel{
			DeviceType:         types.StringValue(fw.FirstBootDeviceType),
			ControllerNumber:   types.Int64Value(int64(fw.FirstBootDeviceControllerNumber)),
			ControllerLocation: types.Int64Value(int64(fw.FirstBootDeviceControllerLocation)),
		})
		diags.Append(d...)
		model.FirstBootDevice = fbd
	} else {
		model.FirstBootDevice = types.ObjectNull(firstBootDeviceAttrTypes())
	}
}

func firstBootDeviceAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"device_type":         types.StringType,
		"controller_number":   types.Int64Type,
		"controller_location": types.Int64Type,
	}
}

func mapVMToState(vm *client.VM, model *vmResourceModel, diags *diag.Diagnostics) {
	model.Name = types.StringValue(vm.Name)
	model.Generation = types.Int64Value(int64(vm.Generation))
	model.ProcessorCount = types.Int64Value(int64(vm.ProcessorCount))
	model.MemoryStartupBytes = types.Int64Value(vm.MemoryStartup)
	model.MemoryMinimumBytes = types.Int64Value(vm.MemoryMinimum)
	model.MemoryMaximumBytes = types.Int64Value(vm.MemoryMaximum)
	model.DynamicMemory = types.BoolValue(vm.DynamicMemoryEnabled)
	model.Notes = types.StringValue(vm.Notes)

	switch vm.State {
	case "Running":
		model.State = types.StringValue("Running")
	case "Off":
		model.State = types.StringValue("Off")
	default:
		model.State = types.StringValue("Off")
		diags.AddWarning(
			"VM in unexpected state",
			fmt.Sprintf("VM %q is in Hyper-V state %q (not Running or Off), treating as Off.", vm.Name, vm.State),
		)
	}

	model.AutomaticStartAction = types.StringValue(vm.AutomaticStartAction)
	model.AutomaticStopAction = types.StringValue(vm.AutomaticStopAction)
	model.CheckpointType = types.StringValue(vm.CheckpointType)
}
