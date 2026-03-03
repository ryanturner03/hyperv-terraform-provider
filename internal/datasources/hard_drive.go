package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var _ datasource.DataSource = &hardDriveDataSource{}

type hardDriveDataSource struct {
	client client.HyperVClient
}

type hardDriveDataSourceModel struct {
	VMName             types.String `tfsdk:"vm_name"`
	ControllerType     types.String `tfsdk:"controller_type"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
	Path               types.String `tfsdk:"path"`
}

func NewHardDriveDataSource() datasource.DataSource {
	return &hardDriveDataSource{}
}

func (d *hardDriveDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hard_drive"
}

func (d *hardDriveDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about a Hyper-V virtual hard disk drive attached to a VM.",
		Attributes: map[string]schema.Attribute{
			"vm_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine.",
			},
			"controller_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The controller type: IDE or SCSI.",
			},
			"controller_number": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The controller number.",
			},
			"controller_location": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The controller location (slot).",
			},
			"path": schema.StringAttribute{
				Computed:    true,
				Description: "The path to the attached VHD/VHDX file, if any.",
			},
		},
	}
}

func (d *hardDriveDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *hardDriveDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config hardDriveDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmName := config.VMName.ValueString()

	// Direct lookup when controller_type, controller_number, and controller_location are all specified
	if !config.ControllerType.IsNull() && !config.ControllerNumber.IsNull() && !config.ControllerLocation.IsNull() {
		drive, err := d.client.GetHardDrive(
			ctx,
			vmName,
			config.ControllerType.ValueString(),
			int(config.ControllerNumber.ValueInt64()),
			int(config.ControllerLocation.ValueInt64()),
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading hard drive",
				fmt.Sprintf("Could not read hard drive on VM %q (%s %d:%d): %s",
					vmName, config.ControllerType.ValueString(), config.ControllerNumber.ValueInt64(), config.ControllerLocation.ValueInt64(), err.Error()),
			)
			return
		}

		mapHardDriveDataToState(drive, &config)
		resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
		return
	}

	// List and filter
	drives, err := d.client.ListHardDrives(ctx, vmName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing hard drives",
			fmt.Sprintf("Could not list hard drives on VM %q: %s", vmName, err.Error()),
		)
		return
	}

	var matches []client.HardDrive
	for _, drive := range drives {
		if !config.ControllerType.IsNull() && controllerTypeToString(drive.ControllerType) != config.ControllerType.ValueString() {
			continue
		}
		if !config.ControllerNumber.IsNull() && int64(drive.ControllerNumber) != config.ControllerNumber.ValueInt64() {
			continue
		}
		if !config.ControllerLocation.IsNull() && int64(drive.ControllerLocation) != config.ControllerLocation.ValueInt64() {
			continue
		}
		matches = append(matches, drive)
	}

	if len(matches) == 0 {
		resp.Diagnostics.AddError(
			"No hard drive found",
			fmt.Sprintf("No hard drive found on VM %q matching the specified criteria.", vmName),
		)
		return
	}
	if len(matches) > 1 {
		resp.Diagnostics.AddError(
			"Multiple hard drives found",
			fmt.Sprintf("Found %d hard drives on VM %q matching the specified criteria. Specify controller_type, controller_number, and controller_location to select one.", len(matches), vmName),
		)
		return
	}

	mapHardDriveDataToState(&matches[0], &config)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func controllerTypeToString(ct int) string {
	switch ct {
	case 0:
		return "IDE"
	default:
		return "SCSI"
	}
}

func mapHardDriveDataToState(drive *client.HardDrive, model *hardDriveDataSourceModel) {
	model.VMName = types.StringValue(drive.VMName)
	model.ControllerType = types.StringValue(controllerTypeToString(drive.ControllerType))
	model.ControllerNumber = types.Int64Value(int64(drive.ControllerNumber))
	model.ControllerLocation = types.Int64Value(int64(drive.ControllerLocation))

	if drive.Path != "" {
		model.Path = types.StringValue(drive.Path)
	} else {
		model.Path = types.StringValue("")
	}
}
