package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var _ datasource.DataSource = &vmDataSource{}

type vmDataSource struct {
	client client.HyperVClient
}

type vmDataSourceModel struct {
	Name               types.String `tfsdk:"name"`
	Generation         types.Int64  `tfsdk:"generation"`
	ProcessorCount     types.Int64  `tfsdk:"processor_count"`
	MemoryStartupBytes types.Int64  `tfsdk:"memory_startup_bytes"`
	MemoryMinimumBytes types.Int64  `tfsdk:"memory_minimum_bytes"`
	MemoryMaximumBytes types.Int64  `tfsdk:"memory_maximum_bytes"`
	DynamicMemory      types.Bool   `tfsdk:"dynamic_memory"`
	State              types.String `tfsdk:"state"`
	Notes              types.String `tfsdk:"notes"`
	SecureBootEnabled  types.Bool   `tfsdk:"secure_boot_enabled"`
	SecureBootTemplate types.String `tfsdk:"secure_boot_template"`
	FirstBootDevice    types.Object `tfsdk:"first_boot_device"`
}

func NewVMDataSource() datasource.DataSource {
	return &vmDataSource{}
}

func (d *vmDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm"
}

func (d *vmDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about a Hyper-V virtual machine.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine.",
			},
			"generation": schema.Int64Attribute{
				Computed:    true,
				Description: "The generation of the virtual machine (1 or 2).",
			},
			"processor_count": schema.Int64Attribute{
				Computed:    true,
				Description: "The number of virtual processors assigned to the VM.",
			},
			"memory_startup_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "The startup memory in bytes.",
			},
			"memory_minimum_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "The minimum memory in bytes when dynamic memory is enabled.",
			},
			"memory_maximum_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "The maximum memory in bytes when dynamic memory is enabled.",
			},
			"dynamic_memory": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether dynamic memory is enabled.",
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "The current state of the VM: Running or Off.",
			},
			"notes": schema.StringAttribute{
				Computed:    true,
				Description: "Notes or description for the virtual machine.",
			},
			"secure_boot_enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Secure Boot is enabled (Generation 2 only).",
			},
			"secure_boot_template": schema.StringAttribute{
				Computed:    true,
				Description: "The Secure Boot template (Generation 2 only).",
			},
			"first_boot_device": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "The first boot device (Generation 2 only).",
				Attributes: map[string]schema.Attribute{
					"device_type": schema.StringAttribute{
						Computed:    true,
						Description: "The type of boot device: HardDiskDrive, DvdDrive, or NetworkAdapter.",
					},
					"controller_number": schema.Int64Attribute{
						Computed:    true,
						Description: "The controller number.",
					},
					"controller_location": schema.Int64Attribute{
						Computed:    true,
						Description: "The controller location.",
					},
				},
			},
		},
	}
}

func (d *vmDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(client.HyperVClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected client.HyperVClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = c
}

func (d *vmDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vmDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vm, err := d.client.GetVM(ctx, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VM",
			fmt.Sprintf("Could not read VM %q: %s", config.Name.ValueString(), err.Error()),
		)
		return
	}

	config.Name = types.StringValue(vm.Name)
	config.Generation = types.Int64Value(int64(vm.Generation))
	config.ProcessorCount = types.Int64Value(int64(vm.ProcessorCount))
	config.MemoryStartupBytes = types.Int64Value(vm.MemoryStartup)
	config.MemoryMinimumBytes = types.Int64Value(vm.MemoryMinimum)
	config.MemoryMaximumBytes = types.Int64Value(vm.MemoryMaximum)
	config.DynamicMemory = types.BoolValue(vm.DynamicMemoryEnabled)
	config.Notes = types.StringValue(vm.Notes)

	if vm.State == "Running" {
		config.State = types.StringValue("Running")
	} else {
		config.State = types.StringValue("Off")
	}

	fbdAttrTypes := map[string]attr.Type{
		"device_type":         types.StringType,
		"controller_number":   types.Int64Type,
		"controller_location": types.Int64Type,
	}

	if vm.Generation == 2 {
		fw, err := d.client.GetVMFirmware(ctx, vm.Name)
		if err != nil {
			resp.Diagnostics.AddWarning("Error reading VM firmware", err.Error())
			config.SecureBootEnabled = types.BoolNull()
			config.SecureBootTemplate = types.StringNull()
			config.FirstBootDevice = types.ObjectNull(fbdAttrTypes)
		} else {
			config.SecureBootEnabled = types.BoolValue(fw.SecureBootEnabled == "On")
			config.SecureBootTemplate = types.StringValue(fw.SecureBootTemplate)

			if fw.FirstBootDeviceType != "" && fw.FirstBootDeviceType != "Unknown" {
				fbd, diags := types.ObjectValueFrom(ctx, fbdAttrTypes, struct {
					DeviceType         types.String `tfsdk:"device_type"`
					ControllerNumber   types.Int64  `tfsdk:"controller_number"`
					ControllerLocation types.Int64  `tfsdk:"controller_location"`
				}{
					DeviceType:         types.StringValue(fw.FirstBootDeviceType),
					ControllerNumber:   types.Int64Value(int64(fw.FirstBootDeviceControllerNumber)),
					ControllerLocation: types.Int64Value(int64(fw.FirstBootDeviceControllerLocation)),
				})
				resp.Diagnostics.Append(diags...)
				config.FirstBootDevice = fbd
			} else {
				config.FirstBootDevice = types.ObjectNull(fbdAttrTypes)
			}
		}
	} else {
		config.SecureBootEnabled = types.BoolNull()
		config.SecureBootTemplate = types.StringNull()
		config.FirstBootDevice = types.ObjectNull(fbdAttrTypes)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
