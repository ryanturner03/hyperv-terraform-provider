package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource              = &virtualSwitchResource{}
	_ resource.ResourceWithConfigure = &virtualSwitchResource{}
)

type virtualSwitchResource struct {
	client client.HyperVClient
}

type virtualSwitchResourceModel struct {
	Name              types.String `tfsdk:"name"`
	Type              types.String `tfsdk:"type"`
	NetAdapterName    types.String `tfsdk:"net_adapter_name"`
	AllowManagementOS types.Bool   `tfsdk:"allow_management_os"`
}

func NewVirtualSwitchResource() resource.Resource {
	return &virtualSwitchResource{}
}

func (r *virtualSwitchResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_switch"
}

func (r *virtualSwitchResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Hyper-V virtual switch.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual switch.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the virtual switch: External, Internal, or Private. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("External", "Internal", "Private"),
				},
			},
			"net_adapter_name": schema.StringAttribute{
				Optional:    true,
				Description: "The name of the physical network adapter to bind to (required for External switches).",
			},
			"allow_management_os": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether to allow the management OS to share the adapter. Defaults depend on switch type.",
			},
		},
	}
}

func (r *virtualSwitchResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *virtualSwitchResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan virtualSwitchResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.SwitchOptions{
		Name:       plan.Name.ValueString(),
		SwitchType: plan.Type.ValueString(),
	}
	if !plan.AllowManagementOS.IsNull() && !plan.AllowManagementOS.IsUnknown() {
		opts.AllowManagementOS = plan.AllowManagementOS.ValueBool()
		opts.AllowManagementOSSet = true
	}
	if !plan.NetAdapterName.IsNull() {
		opts.NetAdapterName = plan.NetAdapterName.ValueString()
	}

	sw, err := r.client.CreateVirtualSwitch(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating virtual switch", err.Error())
		return
	}

	mapVirtualSwitchToState(sw, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *virtualSwitchResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state virtualSwitchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sw, err := r.client.GetVirtualSwitch(ctx, state.Name.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "ObjectNotFound") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading virtual switch", err.Error())
		return
	}

	mapVirtualSwitchToState(sw, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *virtualSwitchResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan virtualSwitchResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.SwitchOptions{
		Name:       plan.Name.ValueString(),
		SwitchType: plan.Type.ValueString(),
	}
	if !plan.AllowManagementOS.IsNull() && !plan.AllowManagementOS.IsUnknown() {
		opts.AllowManagementOS = plan.AllowManagementOS.ValueBool()
		opts.AllowManagementOSSet = true
	}
	if !plan.NetAdapterName.IsNull() {
		opts.NetAdapterName = plan.NetAdapterName.ValueString()
	}

	err := r.client.UpdateVirtualSwitch(ctx, plan.Name.ValueString(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Error updating virtual switch", err.Error())
		return
	}

	sw, err := r.client.GetVirtualSwitch(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading virtual switch after update", err.Error())
		return
	}

	mapVirtualSwitchToState(sw, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *virtualSwitchResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state virtualSwitchResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteVirtualSwitch(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting virtual switch", err.Error())
		return
	}
}

func mapVirtualSwitchToState(sw *client.VirtualSwitch, model *virtualSwitchResourceModel) {
	model.Name = types.StringValue(sw.Name)
	model.AllowManagementOS = types.BoolValue(sw.AllowManagementOS)

	// Map SwitchType int to string
	switch sw.SwitchType {
	case 0:
		model.Type = types.StringValue("External")
	case 1:
		model.Type = types.StringValue("Internal")
	case 2:
		model.Type = types.StringValue("Private")
	default:
		model.Type = types.StringValue("Private")
	}

	if sw.NetAdapterName != "" {
		model.NetAdapterName = types.StringValue(sw.NetAdapterName)
	}
}
