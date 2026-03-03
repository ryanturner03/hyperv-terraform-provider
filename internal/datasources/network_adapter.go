package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var _ datasource.DataSource = &networkAdapterDataSource{}

type networkAdapterDataSource struct {
	client client.HyperVClient
}

type networkAdapterDataSourceModel struct {
	Name              types.String `tfsdk:"name"`
	VMName            types.String `tfsdk:"vm_name"`
	SwitchName        types.String `tfsdk:"switch_name"`
	MacAddress        types.String `tfsdk:"mac_address"`
	DynamicMacAddress types.Bool   `tfsdk:"dynamic_mac_address"`
}

func NewNetworkAdapterDataSource() datasource.DataSource {
	return &networkAdapterDataSource{}
}

func (d *networkAdapterDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_adapter"
}

func (d *networkAdapterDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about a Hyper-V virtual network adapter attached to a VM.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the network adapter.",
			},
			"vm_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual machine the adapter is attached to.",
			},
			"switch_name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the virtual switch the adapter is connected to.",
			},
			"mac_address": schema.StringAttribute{
				Computed:    true,
				Description: "The MAC address of the network adapter.",
			},
			"dynamic_mac_address": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the MAC address is dynamically assigned.",
			},
		},
	}
}

func (d *networkAdapterDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *networkAdapterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config networkAdapterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	adapter, err := d.client.GetNetworkAdapter(ctx, config.VMName.ValueString(), config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading network adapter",
			fmt.Sprintf("Could not read network adapter %q on VM %q: %s",
				config.Name.ValueString(), config.VMName.ValueString(), err.Error()),
		)
		return
	}

	config.Name = types.StringValue(adapter.Name)
	config.VMName = types.StringValue(adapter.VMName)
	config.MacAddress = types.StringValue(adapter.MacAddress)
	config.DynamicMacAddress = types.BoolValue(adapter.DynamicMacAddress)

	if adapter.SwitchName != "" {
		config.SwitchName = types.StringValue(adapter.SwitchName)
	} else {
		config.SwitchName = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
