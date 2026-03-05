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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
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
	HardDrives           types.List   `tfsdk:"hard_drive"`
	DVDDrives            types.List   `tfsdk:"dvd_drive"`
	NetworkAdapters      types.List   `tfsdk:"network_adapter"`
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
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"memory_maximum_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The maximum memory in bytes when dynamic memory is enabled.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"automatic_start_action": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The action to take when the host starts (Nothing, Start, StartIfRunning).",
				Validators: []validator.String{
					stringvalidator.OneOf("Nothing", "StartIfRunning", "Start"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"automatic_stop_action": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The action to take when the host shuts down (TurnOff, Save, ShutDown).",
				Validators: []validator.String{
					stringvalidator.OneOf("TurnOff", "Save", "ShutDown"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"checkpoint_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The checkpoint type (Disabled, Production, ProductionOnly, Standard).",
				Validators: []validator.String{
					stringvalidator.OneOf("Disabled", "Production", "ProductionOnly", "Standard"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"secure_boot_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether Secure Boot is enabled (Generation 2 only).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"secure_boot_template": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The Secure Boot template (Generation 2 only). Values: MicrosoftWindows, MicrosoftUEFICertificateAuthority.",
				Validators: []validator.String{
					stringvalidator.OneOf("MicrosoftWindows", "MicrosoftUEFICertificateAuthority"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"first_boot_device": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The first boot device (Generation 2 only). Note: the referenced drive must already exist on the VM.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
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
		Blocks: map[string]schema.Block{
			"hard_drive": schema.ListNestedBlock{
				Description: "Inline hard drive attached to the VM. Created atomically with the VM.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Path to the VHD/VHDX file.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"controller_type": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("SCSI"),
							Description: "Controller type: IDE or SCSI (default: SCSI).",
							Validators: []validator.String{
								stringvalidator.OneOf("IDE", "SCSI"),
							},
						},
						"controller_number": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Default:     int64default.StaticInt64(0),
							Description: "Controller number (default: 0).",
						},
						"controller_location": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Controller location. Auto-assigned if omitted.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"dvd_drive": schema.ListNestedBlock{
				Description: "Inline DVD drive attached to the VM. Created atomically with the VM.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"path": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Path to the ISO file.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"controller_number": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Default:     int64default.StaticInt64(0),
							Description: "Controller number (default: 0).",
						},
						"controller_location": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Controller location. Auto-assigned if omitted.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"network_adapter": schema.ListNestedBlock{
				Description: "Inline network adapter attached to the VM. Created atomically with the VM.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the network adapter.",
						},
						"switch_name": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Name of the virtual switch to connect to.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"vlan_id": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "VLAN ID for the adapter.",
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						"mac_address": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "MAC address. Assigned by Hyper-V when the VM starts if dynamic.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"dynamic_mac_address": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(true),
							Description: "Whether to use a dynamic MAC address (default: true).",
						},
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

	// Determine if inline blocks are present
	hasInlineBlocks := !plan.HardDrives.IsNull() || !plan.DVDDrives.IsNull() || !plan.NetworkAdapters.IsNull()

	// Create inline hard drives
	var createdHardDrives []inlineHardDriveModel
	if !plan.HardDrives.IsNull() {
		var planned []inlineHardDriveModel
		resp.Diagnostics.Append(plan.HardDrives.ElementsAs(ctx, &planned, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var err error
		createdHardDrives, err = createInlineHardDrives(ctx, r.client, vmName, planned)
		if err != nil {
			_ = r.client.DeleteVM(ctx, vmName)
			resp.Diagnostics.AddError("Error creating inline hard drive", err.Error())
			return
		}
	}

	// Create inline DVD drives
	var createdDVDDrives []inlineDVDDriveModel
	if !plan.DVDDrives.IsNull() {
		var planned []inlineDVDDriveModel
		resp.Diagnostics.Append(plan.DVDDrives.ElementsAs(ctx, &planned, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var err error
		createdDVDDrives, err = createInlineDVDDrives(ctx, r.client, vmName, planned)
		if err != nil {
			_ = r.client.DeleteVM(ctx, vmName)
			resp.Diagnostics.AddError("Error creating inline DVD drive", err.Error())
			return
		}
	}

	// Create inline network adapters
	var createdNetworkAdapters []inlineNetworkAdapterModel
	if !plan.NetworkAdapters.IsNull() {
		var planned []inlineNetworkAdapterModel
		resp.Diagnostics.Append(plan.NetworkAdapters.ElementsAs(ctx, &planned, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		var err error
		createdNetworkAdapters, err = createInlineNetworkAdapters(ctx, r.client, vmName, planned)
		if err != nil {
			_ = r.client.DeleteVM(ctx, vmName)
			resp.Diagnostics.AddError("Error creating inline network adapter", err.Error())
			return
		}
	}

	// Configure firmware for Gen 2 VMs (secure boot, template, first boot device).
	// Everything goes in a single Set-VMFirmware call to avoid the problem where
	// separate invocations reset each other's settings.
	if isGen2 {
		fwOpts := r.buildFirmwareOpts(plan)
		if fwOpts == nil {
			fwOpts = &client.VMFirmwareOptions{}
		}

		if !plan.FirstBootDevice.IsNull() && !plan.FirstBootDevice.IsUnknown() {
			var fbd firstBootDeviceModel
			resp.Diagnostics.Append(plan.FirstBootDevice.As(ctx, &fbd, basetypes.ObjectAsOptions{})...)
			if resp.Diagnostics.HasError() {
				return
			}
			fwOpts.FirstBootDevice = &client.BootDevice{
				DeviceType:         fbd.DeviceType.ValueString(),
				ControllerNumber:   int(fbd.ControllerNumber.ValueInt64()),
				ControllerLocation: int(fbd.ControllerLocation.ValueInt64()),
			}
		}

		if fwOpts.SecureBootEnabled != nil || fwOpts.SecureBootTemplate != "" || fwOpts.FirstBootDevice != nil {
			if err := r.client.SetVMFirmware(ctx, vmName, *fwOpts); err != nil {
				if fwOpts.FirstBootDevice != nil && strings.Contains(err.Error(), "not attached yet") {
					d := fwOpts.FirstBootDevice
					resp.Diagnostics.AddWarning(
						"First boot device not set — drive not attached yet",
						fmt.Sprintf("The drive at controller %d:%d does not exist on the VM yet. "+
							"This is expected when drive resources are created separately. "+
							"The boot device will be configured on the next terraform apply after drives are attached.",
							d.ControllerNumber, d.ControllerLocation),
					)
					// Retry without the boot device so secure boot still gets set
					fwOpts.FirstBootDevice = nil
					if fwOpts.SecureBootEnabled != nil || fwOpts.SecureBootTemplate != "" {
						if err2 := r.client.SetVMFirmware(ctx, vmName, *fwOpts); err2 != nil {
							_ = r.client.DeleteVM(ctx, vmName)
							resp.Diagnostics.AddError("Error configuring VM firmware", err2.Error())
							return
						}
					}
				} else {
					_ = r.client.DeleteVM(ctx, vmName)
					resp.Diagnostics.AddError("Error configuring VM firmware", err.Error())
					return
				}
			}
		}
	}

	// Warn when starting during Create — drives likely aren't attached yet
	// Suppress the warning when inline blocks are present (drives are already attached)
	if plan.State.ValueString() == "Running" && !hasInlineBlocks {
		resp.Diagnostics.AddWarning(
			"VM starting before drives may be attached",
			"Drives managed as separate resources may not exist yet. "+
				"Consider state = \"Off\", then change to \"Running\" after drives are attached.",
		)
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
		plannedFBD := plan.FirstBootDevice
		r.readFirmwareState(ctx, vmName, &plan, &resp.Diagnostics)
		// Preserve the planned first_boot_device value. We just set it via
		// SetVMFirmware, so we know it's correct. The read-back can return
		// Unknown/null due to WinRM deserialization issues with type detection.
		if !plannedFBD.IsNull() {
			plan.FirstBootDevice = plannedFBD
		}
	}

	// Read back inline sub-resources from server (not from create results)
	// to capture computed fields that may change after VM start (e.g., MAC address)
	if createdHardDrives != nil {
		drives, err := r.client.ListHardDrives(ctx, vmName)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline hard drives", err.Error())
			plan.HardDrives = hardDrivesToList(ctx, createdHardDrives, &resp.Diagnostics)
		} else {
			plan.HardDrives = hardDrivesToList(ctx, hardDrivesFromClient(drives), &resp.Diagnostics)
		}
	} else {
		plan.HardDrives = hardDrivesToList(ctx, nil, &resp.Diagnostics)
	}
	if createdDVDDrives != nil {
		drives, err := r.client.ListDVDDrives(ctx, vmName)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline DVD drives", err.Error())
			plan.DVDDrives = dvdDrivesToList(ctx, createdDVDDrives, &resp.Diagnostics)
		} else {
			plan.DVDDrives = dvdDrivesToList(ctx, dvdDrivesFromClient(drives), &resp.Diagnostics)
		}
	} else {
		plan.DVDDrives = dvdDrivesToList(ctx, nil, &resp.Diagnostics)
	}
	if createdNetworkAdapters != nil {
		adapters, err := r.client.ListNetworkAdapters(ctx, vmName)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline network adapters", err.Error())
			plan.NetworkAdapters = networkAdaptersToList(ctx, createdNetworkAdapters, &resp.Diagnostics)
		} else {
			plan.NetworkAdapters = networkAdaptersToList(ctx, networkAdaptersFromClient(adapters), &resp.Diagnostics)
		}
	} else {
		plan.NetworkAdapters = networkAdaptersToList(ctx, nil, &resp.Diagnostics)
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
		priorFBD := state.FirstBootDevice
		r.readFirmwareState(ctx, vm.Name, &state, &resp.Diagnostics)
		// If the read-back couldn't determine the boot device type (WinRM
		// deserialization drops type info), preserve the prior state value.
		if state.FirstBootDevice.IsNull() && !priorFBD.IsNull() {
			state.FirstBootDevice = priorFBD
		}
	} else {
		state.SecureBootEnabled = types.BoolNull()
		state.SecureBootTemplate = types.StringNull()
		state.FirstBootDevice = types.ObjectNull(firstBootDeviceAttrTypes())
	}

	// Read back inline sub-resources only if state has non-null lists
	// (prevents absorbing drives from standalone resources)
	if !state.HardDrives.IsNull() {
		drives, err := r.client.ListHardDrives(ctx, vm.Name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline hard drives", err.Error())
		} else {
			state.HardDrives = hardDrivesToList(ctx, hardDrivesFromClient(drives), &resp.Diagnostics)
		}
	}
	if !state.DVDDrives.IsNull() {
		drives, err := r.client.ListDVDDrives(ctx, vm.Name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline DVD drives", err.Error())
		} else {
			state.DVDDrives = dvdDrivesToList(ctx, dvdDrivesFromClient(drives), &resp.Diagnostics)
		}
	}
	if !state.NetworkAdapters.IsNull() {
		adapters, err := r.client.ListNetworkAdapters(ctx, vm.Name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline network adapters", err.Error())
		} else {
			state.NetworkAdapters = networkAdaptersToList(ctx, networkAdaptersFromClient(adapters), &resp.Diagnostics)
		}
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

	// Firmware, first boot device, and inline block changes may require the VM to be off.
	needsFirmware := isGen2 && r.buildFirmwareOpts(plan) != nil
	needsBootDevice := isGen2 && !plan.FirstBootDevice.IsNull() && !plan.FirstBootDevice.IsUnknown()
	needsInlineChanges := !plan.HardDrives.IsNull() || !plan.DVDDrives.IsNull() || !plan.NetworkAdapters.IsNull()
	stoppedForConfig := false

	if currentState == "Running" && (needsFirmware || needsBootDevice || needsInlineChanges) {
		if err := r.client.SetVMState(ctx, name, client.VMStateOff); err != nil {
			resp.Diagnostics.AddError("Error stopping VM for config changes", err.Error())
			return
		}
		stoppedForConfig = true
	}

	err := r.client.UpdateVM(ctx, name, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error updating VM", err.Error())
		return
	}

	// Diff and apply inline sub-resources
	if !plan.HardDrives.IsNull() || !state.HardDrives.IsNull() {
		var oldDrives, newDrives []inlineHardDriveModel
		if !state.HardDrives.IsNull() {
			resp.Diagnostics.Append(state.HardDrives.ElementsAs(ctx, &oldDrives, false)...)
		}
		if !plan.HardDrives.IsNull() {
			resp.Diagnostics.Append(plan.HardDrives.ElementsAs(ctx, &newDrives, false)...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
		if err := diffAndApplyHardDrives(ctx, r.client, name, oldDrives, newDrives); err != nil {
			resp.Diagnostics.AddError("Error updating inline hard drives", err.Error())
			return
		}
	}

	if !plan.DVDDrives.IsNull() || !state.DVDDrives.IsNull() {
		var oldDrives, newDrives []inlineDVDDriveModel
		if !state.DVDDrives.IsNull() {
			resp.Diagnostics.Append(state.DVDDrives.ElementsAs(ctx, &oldDrives, false)...)
		}
		if !plan.DVDDrives.IsNull() {
			resp.Diagnostics.Append(plan.DVDDrives.ElementsAs(ctx, &newDrives, false)...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
		if err := diffAndApplyDVDDrives(ctx, r.client, name, oldDrives, newDrives); err != nil {
			resp.Diagnostics.AddError("Error updating inline DVD drives", err.Error())
			return
		}
	}

	if !plan.NetworkAdapters.IsNull() || !state.NetworkAdapters.IsNull() {
		var oldAdapters, newAdapters []inlineNetworkAdapterModel
		if !state.NetworkAdapters.IsNull() {
			resp.Diagnostics.Append(state.NetworkAdapters.ElementsAs(ctx, &oldAdapters, false)...)
		}
		if !plan.NetworkAdapters.IsNull() {
			resp.Diagnostics.Append(plan.NetworkAdapters.ElementsAs(ctx, &newAdapters, false)...)
		}
		if resp.Diagnostics.HasError() {
			return
		}
		if err := diffAndApplyNetworkAdapters(ctx, r.client, name, oldAdapters, newAdapters); err != nil {
			resp.Diagnostics.AddError("Error updating inline network adapters", err.Error())
			return
		}
	}

	// Configure firmware for Gen 2 VMs (secure boot, template, first boot device)
	// in a single Set-VMFirmware call to avoid settings resetting each other.
	if needsFirmware || needsBootDevice {
		fwOpts := r.buildFirmwareOpts(plan)
		if fwOpts == nil {
			fwOpts = &client.VMFirmwareOptions{}
		}

		if needsBootDevice {
			var fbd firstBootDeviceModel
			resp.Diagnostics.Append(plan.FirstBootDevice.As(ctx, &fbd, basetypes.ObjectAsOptions{})...)
			if resp.Diagnostics.HasError() {
				return
			}
			fwOpts.FirstBootDevice = &client.BootDevice{
				DeviceType:         fbd.DeviceType.ValueString(),
				ControllerNumber:   int(fbd.ControllerNumber.ValueInt64()),
				ControllerLocation: int(fbd.ControllerLocation.ValueInt64()),
			}
		}

		if err := r.client.SetVMFirmware(ctx, name, *fwOpts); err != nil {
			resp.Diagnostics.AddError("Error configuring VM firmware", err.Error())
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

	// Read back inline sub-resources into state
	if !plan.HardDrives.IsNull() {
		drives, err := r.client.ListHardDrives(ctx, name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline hard drives", err.Error())
		} else {
			plan.HardDrives = hardDrivesToList(ctx, hardDrivesFromClient(drives), &resp.Diagnostics)
		}
	}
	if !plan.DVDDrives.IsNull() {
		drives, err := r.client.ListDVDDrives(ctx, name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline DVD drives", err.Error())
		} else {
			plan.DVDDrives = dvdDrivesToList(ctx, dvdDrivesFromClient(drives), &resp.Diagnostics)
		}
	}
	if !plan.NetworkAdapters.IsNull() {
		adapters, err := r.client.ListNetworkAdapters(ctx, name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading inline network adapters", err.Error())
		} else {
			plan.NetworkAdapters = networkAdaptersToList(ctx, networkAdaptersFromClient(adapters), &resp.Diagnostics)
		}
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
