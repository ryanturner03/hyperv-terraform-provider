package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource              = &networkAdapterResource{}
	_ resource.ResourceWithConfigure = &networkAdapterResource{}
)

type networkAdapterResource struct {
	client client.HyperVClient
}

type networkAdapterResourceModel struct {
	Name              types.String `tfsdk:"name"`
	VMName            types.String `tfsdk:"vm_name"`
	SwitchName        types.String `tfsdk:"switch_name"`
	VlanID            types.Int64  `tfsdk:"vlan_id"`
	MacAddress        types.String `tfsdk:"mac_address"`
	DynamicMacAddress types.Bool   `tfsdk:"dynamic_mac_address"`
}

func NewNetworkAdapterResource() resource.Resource {
	return &networkAdapterResource{}
}

func (r *networkAdapterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_adapter"
}

func (r *networkAdapterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Hyper-V virtual network adapter attached to a VM.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the network adapter.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vm_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine to attach the adapter to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"switch_name": schema.StringAttribute{
				Optional:    true,
				Description: "The name of the virtual switch to connect the adapter to.",
			},
			"vlan_id": schema.Int64Attribute{
				Optional:    true,
				Description: "The VLAN ID for the network adapter.",
			},
			"mac_address": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The MAC address of the network adapter. Computed if not specified.",
			},
			"dynamic_mac_address": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the MAC address is dynamically assigned.",
			},
		},
	}
}

func (r *networkAdapterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *networkAdapterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkAdapterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.AdapterOptions{
		Name:              plan.Name.ValueString(),
		VMName:            plan.VMName.ValueString(),
		DynamicMacAddress: plan.DynamicMacAddress.ValueBool(),
	}
	if !plan.SwitchName.IsNull() {
		opts.SwitchName = plan.SwitchName.ValueString()
	}
	if !plan.VlanID.IsNull() {
		opts.VlanID = int(plan.VlanID.ValueInt64())
		opts.VlanIDSet = true
	}
	if !plan.MacAddress.IsNull() && !plan.MacAddress.IsUnknown() {
		opts.MacAddress = plan.MacAddress.ValueString()
	}

	adapter, err := r.client.CreateNetworkAdapter(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network adapter", err.Error())
		return
	}

	mapNetworkAdapterToState(adapter, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkAdapterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkAdapterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	adapter, err := r.client.GetNetworkAdapter(ctx, state.VMName.ValueString(), state.Name.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "ObjectNotFound") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading network adapter", err.Error())
		return
	}

	mapNetworkAdapterToState(adapter, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkAdapterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkAdapterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.AdapterOptions{
		Name:              plan.Name.ValueString(),
		VMName:            plan.VMName.ValueString(),
		DynamicMacAddress: plan.DynamicMacAddress.ValueBool(),
	}
	if !plan.SwitchName.IsNull() {
		opts.SwitchName = plan.SwitchName.ValueString()
	}
	if !plan.VlanID.IsNull() {
		opts.VlanID = int(plan.VlanID.ValueInt64())
		opts.VlanIDSet = true
	}

	err := r.client.UpdateNetworkAdapter(ctx, plan.VMName.ValueString(), plan.Name.ValueString(), opts)
	if err != nil {
		resp.Diagnostics.AddError("Error updating network adapter", err.Error())
		return
	}

	adapter, err := r.client.GetNetworkAdapter(ctx, plan.VMName.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading network adapter after update", err.Error())
		return
	}

	mapNetworkAdapterToState(adapter, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkAdapterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkAdapterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteNetworkAdapter(ctx, state.VMName.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting network adapter", err.Error())
		return
	}
}

func mapNetworkAdapterToState(adapter *client.NetworkAdapter, model *networkAdapterResourceModel) {
	model.Name = types.StringValue(adapter.Name)
	model.VMName = types.StringValue(adapter.VMName)
	model.MacAddress = types.StringValue(adapter.MacAddress)
	model.DynamicMacAddress = types.BoolValue(adapter.DynamicMacAddress)

	if adapter.SwitchName != "" {
		model.SwitchName = types.StringValue(adapter.SwitchName)
	} else {
		model.SwitchName = types.StringNull()
	}

	if adapter.VlanID > 0 {
		model.VlanID = types.Int64Value(int64(adapter.VlanID))
	} else {
		model.VlanID = types.Int64Null()
	}
}
