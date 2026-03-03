package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	schemavalidator "github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource              = &hardDriveResource{}
	_ resource.ResourceWithConfigure = &hardDriveResource{}
)

type hardDriveResource struct {
	client client.HyperVClient
}

type hardDriveResourceModel struct {
	VMName             types.String `tfsdk:"vm_name"`
	Path               types.String `tfsdk:"path"`
	ControllerType     types.String `tfsdk:"controller_type"`
	ControllerNumber   types.Int64  `tfsdk:"controller_number"`
	ControllerLocation types.Int64  `tfsdk:"controller_location"`
}

func NewHardDriveResource() resource.Resource {
	return &hardDriveResource{}
}

func (r *hardDriveResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_hard_drive"
}

func (r *hardDriveResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Hyper-V virtual hard disk drive attached to a VM.",
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
				Description: "The path to the VHD/VHDX file to attach.",
			},
			"controller_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("SCSI"),
				Description: "The controller type: IDE or SCSI. Defaults to SCSI.",
				Validators: []schemavalidator.String{
					stringvalidator.OneOf("IDE", "SCSI"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"controller_number": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "The controller number. Defaults to 0.",
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

func (r *hardDriveResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *hardDriveResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan hardDriveResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.HardDriveOptions{
		VMName:           plan.VMName.ValueString(),
		ControllerType:   plan.ControllerType.ValueString(),
		ControllerNumber: int(plan.ControllerNumber.ValueInt64()),
	}
	if !plan.Path.IsNull() {
		opts.Path = plan.Path.ValueString()
	}
	if !plan.ControllerLocation.IsNull() && !plan.ControllerLocation.IsUnknown() {
		opts.ControllerLocation = int(plan.ControllerLocation.ValueInt64())
		opts.ControllerLocationSet = true
	}

	drive, err := r.client.CreateHardDrive(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating hard drive", err.Error())
		return
	}

	mapHardDriveToState(drive, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hardDriveResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state hardDriveResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	drive, err := r.client.GetHardDrive(
		ctx,
		state.VMName.ValueString(),
		state.ControllerType.ValueString(),
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
		resp.Diagnostics.AddError("Error reading hard drive", err.Error())
		return
	}

	mapHardDriveToState(drive, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *hardDriveResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan hardDriveResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := ""
	if !plan.Path.IsNull() {
		path = plan.Path.ValueString()
	}

	err := r.client.UpdateHardDrive(
		ctx,
		plan.VMName.ValueString(),
		plan.ControllerType.ValueString(),
		int(plan.ControllerNumber.ValueInt64()),
		int(plan.ControllerLocation.ValueInt64()),
		path,
	)
	if err != nil {
		resp.Diagnostics.AddError("Error updating hard drive", err.Error())
		return
	}

	drive, err := r.client.GetHardDrive(
		ctx,
		plan.VMName.ValueString(),
		plan.ControllerType.ValueString(),
		int(plan.ControllerNumber.ValueInt64()),
		int(plan.ControllerLocation.ValueInt64()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error reading hard drive after update", err.Error())
		return
	}

	mapHardDriveToState(drive, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *hardDriveResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state hardDriveResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteHardDrive(
		ctx,
		state.VMName.ValueString(),
		state.ControllerType.ValueString(),
		int(state.ControllerNumber.ValueInt64()),
		int(state.ControllerLocation.ValueInt64()),
	)
	if err != nil {
		resp.Diagnostics.AddError("Error deleting hard drive", err.Error())
		return
	}
}

func mapHardDriveToState(drive *client.HardDrive, model *hardDriveResourceModel) {
	model.VMName = types.StringValue(drive.VMName)
	model.ControllerNumber = types.Int64Value(int64(drive.ControllerNumber))
	model.ControllerLocation = types.Int64Value(int64(drive.ControllerLocation))

	// Map int ControllerType (0=IDE, 1=SCSI) to string
	switch drive.ControllerType {
	case 0:
		model.ControllerType = types.StringValue("IDE")
	default:
		model.ControllerType = types.StringValue("SCSI")
	}

	if drive.Path != "" {
		model.Path = types.StringValue(drive.Path)
	} else {
		model.Path = types.StringNull()
	}
}
