package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource              = &dvdDriveResource{}
	_ resource.ResourceWithConfigure = &dvdDriveResource{}
)

type dvdDriveResource struct {
	client client.HyperVClient
}

type dvdDriveResourceModel struct {
	VMName             types.String `tfsdk:"vm_name"`
	Path               types.String `tfsdk:"path"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
}

func NewDVDDriveResource() resource.Resource {
	return &dvdDriveResource{}
}

func (r *dvdDriveResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dvd_drive"
}

func (r *dvdDriveResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Hyper-V virtual DVD drive attached to a VM.",
		Attributes: map[string]schema.Attribute{
			"vm_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"path": schema.StringAttribute{
				Optional:    true,
				Description: "The path to the ISO file to mount in the DVD drive.",
			},
			"controller_number": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "The IDE/SCSI controller number. Defaults to 0.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"controller_location": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The controller location (slot). Auto-assigned by Hyper-V if not specified.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *dvdDriveResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dvdDriveResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dvdDriveResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.DVDDriveOptions{
		VMName:           plan.VMName.ValueString(),
		ControllerNumber: int(plan.ControllerNumber.ValueInt64()),
	}
	if !plan.Path.IsNull() {
		opts.Path = plan.Path.ValueString()
	}
	if !plan.ControllerLocation.IsNull() && !plan.ControllerLocation.IsUnknown() {
		opts.ControllerLocation = int(plan.ControllerLocation.ValueInt64())
		opts.ControllerLocationSet = true
	}

	drive, err := r.client.CreateDVDDrive(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DVD drive", err.Error())
		return
	}

	mapDVDDriveToState(drive, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dvdDriveResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dvdDriveResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	drive, err := r.client.GetDVDDrive(
		ctx,
		state.VMName.ValueString(),
		int(state.ControllerNumber.ValueInt64()),
		int(state.ControllerLocation.ValueInt64()),
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "ObjectNotFound") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DVD drive", err.Error())
		return
	}

	mapDVDDriveToState(drive, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dvdDriveResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dvdDriveResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.DVDDriveOptions{}
	if !plan.Path.IsNull() {
		opts.Path = plan.Path.ValueString()
	}

	err := r.client.UpdateDVDDrive(
		ctx,
		plan.VMName.ValueString(),
		int(plan.ControllerNumber.ValueInt64()),
		int(plan.ControllerLocation.ValueInt64()),
		opts,
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DVD drive", err.Error())
		return
	}

	drive, err := r.client.GetDVDDrive(
		ctx,
		plan.VMName.ValueString(),
		int(plan.ControllerNumber.ValueInt64()),
		int(plan.ControllerLocation.ValueInt64()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DVD drive after update", err.Error())
		return
	}

	mapDVDDriveToState(drive, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dvdDriveResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dvdDriveResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteDVDDrive(
		ctx,
		state.VMName.ValueString(),
		int(state.ControllerNumber.ValueInt64()),
		int(state.ControllerLocation.ValueInt64()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DVD drive", err.Error())
		return
	}
}

func mapDVDDriveToState(drive *client.DVDDrive, model *dvdDriveResourceModel) {
	model.VMName = types.StringValue(drive.VMName)
	model.ControllerNumber = types.Int64Value(int64(drive.ControllerNumber))
	model.ControllerLocation = types.Int64Value(int64(drive.ControllerLocation))

	if drive.Path != "" {
		model.Path = types.StringValue(drive.Path)
	} else {
		model.Path = types.StringNull()
	}
}
