package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var _ datasource.DataSource = &virtualSwitchDataSource{}

type virtualSwitchDataSource struct {
	client client.HyperVClient
}

type virtualSwitchDataSourceModel struct {
	Name              types.String `tfsdk:"name"`
	Type              types.String `tfsdk:"type"`
	AllowManagementOS types.Bool   `tfsdk:"allow_management_os"`
}

func NewVirtualSwitchDataSource() datasource.DataSource {
	return &virtualSwitchDataSource{}
}

func (d *virtualSwitchDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_switch"
}

func (d *virtualSwitchDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about a Hyper-V virtual switch.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the virtual switch.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The type of the virtual switch: External, Internal, or Private.",
			},
			"allow_management_os": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the management OS is allowed to share the physical adapter.",
			},
		},
	}
}

func (d *virtualSwitchDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *virtualSwitchDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config virtualSwitchDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sw, err := d.client.GetVirtualSwitch(ctx, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading virtual switch",
			fmt.Sprintf("Could not read virtual switch %q: %s", config.Name.ValueString(), err.Error()),
		)
		return
	}

	config.Name = types.StringValue(sw.Name)
	config.AllowManagementOS = types.BoolValue(sw.AllowManagementOS)

	// Map SwitchType int to string
	switch sw.SwitchType {
	case 0:
		config.Type = types.StringValue("External")
	case 1:
		config.Type = types.StringValue("Internal")
	case 2:
		config.Type = types.StringValue("Private")
	default:
		config.Type = types.StringValue("Private")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
