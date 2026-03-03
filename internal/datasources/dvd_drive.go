package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var _ datasource.DataSource = &dvdDriveDataSource{}

type dvdDriveDataSource struct {
	client client.HyperVClient
}

type dvdDriveDataSourceModel struct {
	VMName             types.String `tfsdk:"vm_name"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
	Path               types.String `tfsdk:"path"`
}

func NewDVDDriveDataSource() datasource.DataSource {
	return &dvdDriveDataSource{}
}

func (d *dvdDriveDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dvd_drive"
}

func (d *dvdDriveDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about a Hyper-V virtual DVD drive attached to a VM.",
		Attributes: map[string]schema.Attribute{
			"vm_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine.",
			},
			"controller_number": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The IDE/SCSI controller number.",
			},
			"controller_location": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The controller location (slot).",
			},
			"path": schema.StringAttribute{
				Computed:    true,
				Description: "The path to the mounted ISO file, if any.",
			},
		},
	}
}

func (d *dvdDriveDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *dvdDriveDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config dvdDriveDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vmName := config.VMName.ValueString()

	// Direct lookup when both controller_number and controller_location are specified
	if !config.ControllerNumber.IsNull() && !config.ControllerLocation.IsNull() {
		drive, err := d.client.GetDVDDrive(
			ctx,
			vmName,
			int(config.ControllerNumber.ValueInt64()),
			int(config.ControllerLocation.ValueInt64()),
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading DVD drive",
				fmt.Sprintf("Could not read DVD drive on VM %q (controller %d:%d): %s",
					vmName, config.ControllerNumber.ValueInt64(), config.ControllerLocation.ValueInt64(), err.Error()),
			)
			return
		}

		mapDVDDriveDataToState(drive, &config)
		resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
		return
	}

	// List and filter
	drives, err := d.client.ListDVDDrives(ctx, vmName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error listing DVD drives",
			fmt.Sprintf("Could not list DVD drives on VM %q: %s", vmName, err.Error()),
		)
		return
	}

	var matches []client.DVDDrive
	for _, drive := range drives {
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
			"No DVD drive found",
			fmt.Sprintf("No DVD drive found on VM %q matching the specified criteria.", vmName),
		)
		return
	}
	if len(matches) > 1 {
		resp.Diagnostics.AddError(
			"Multiple DVD drives found",
			fmt.Sprintf("Found %d DVD drives on VM %q matching the specified criteria. Specify controller_number and controller_location to select one.", len(matches), vmName),
		)
		return
	}

	mapDVDDriveDataToState(&matches[0], &config)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func mapDVDDriveDataToState(drive *client.DVDDrive, model *dvdDriveDataSourceModel) {
	model.VMName = types.StringValue(drive.VMName)
	model.ControllerNumber = types.Int64Value(int64(drive.ControllerNumber))
	model.ControllerLocation = types.Int64Value(int64(drive.ControllerLocation))

	if drive.Path != "" {
		model.Path = types.StringValue(drive.Path)
	} else {
		model.Path = types.StringValue("")
	}
}
