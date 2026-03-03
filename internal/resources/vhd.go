package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource              = &vhdResource{}
	_ resource.ResourceWithConfigure = &vhdResource{}
)

type vhdResource struct {
	client client.HyperVClient
}

type vhdResourceModel struct {
	Path           types.String `tfsdk:"path"`
	SizeBytes      types.Int64  `tfsdk:"size_bytes"`
	Type           types.String `tfsdk:"type"`
	ParentPath     types.String `tfsdk:"parent_path"`
	BlockSizeBytes types.Int64  `tfsdk:"block_size_bytes"`
}

func NewVHDResource() resource.Resource {
	return &vhdResource{}
}

func (r *vhdResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vhd"
}

func (r *vhdResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Hyper-V virtual hard disk (VHD/VHDX).",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Required:    true,
				Description: "The file path for the virtual hard disk.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"size_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The size of the virtual hard disk in bytes. Required for Dynamic and Fixed types. Inherited from the parent for Differencing disks.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Dynamic"),
				Description: "The type of the virtual hard disk: Dynamic, Fixed, or Differencing.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("Dynamic", "Fixed", "Differencing"),
				},
			},
			"parent_path": schema.StringAttribute{
				Optional:    true,
				Description: "The parent VHD path for differencing disks.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"block_size_bytes": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The block size of the virtual hard disk in bytes.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *vhdResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vhdResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vhdResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.VHDOptions{
		Path:      plan.Path.ValueString(),
		SizeBytes: plan.SizeBytes.ValueInt64(),
		Type:      plan.Type.ValueString(),
	}
	if !plan.ParentPath.IsNull() {
		opts.ParentPath = plan.ParentPath.ValueString()
	}
	if !plan.BlockSizeBytes.IsNull() && !plan.BlockSizeBytes.IsUnknown() {
		opts.BlockSize = plan.BlockSizeBytes.ValueInt64()
	}

	vhd, err := r.client.CreateVHD(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating VHD", err.Error())
		return
	}

	mapVHDToState(vhd, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vhdResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vhdResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vhd, err := r.client.GetVHD(ctx, state.Path.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "ObjectNotFound") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading VHD", err.Error())
		return
	}

	mapVHDToState(vhd, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vhdResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All attributes require replacement; no in-place updates.
	var plan vhdResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vhdResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vhdResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVHD(ctx, state.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting VHD", err.Error())
		return
	}
}

func mapVHDToState(vhd *client.VHD, model *vhdResourceModel) {
	model.Path = types.StringValue(vhd.Path)
	model.SizeBytes = types.Int64Value(vhd.Size)
	model.BlockSizeBytes = types.Int64Value(vhd.BlockSize)

	// Map VhdType int to string
	switch vhd.VhdType {
	case 2:
		model.Type = types.StringValue("Fixed")
	case 3:
		model.Type = types.StringValue("Dynamic")
	case 4:
		model.Type = types.StringValue("Differencing")
	default:
		model.Type = types.StringValue("Dynamic")
	}

	if vhd.ParentPath != "" {
		model.ParentPath = types.StringValue(vhd.ParentPath)
	} else {
		model.ParentPath = types.StringNull()
	}
}
