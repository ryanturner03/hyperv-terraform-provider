package datasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var _ datasource.DataSource = &vhdDataSource{}

type vhdDataSource struct {
	client client.HyperVClient
}

type vhdDataSourceModel struct {
	Path           types.String `tfsdk:"path"`
	SizeBytes      types.Int64  `tfsdk:"size_bytes"`
	Type           types.String `tfsdk:"type"`
	ParentPath     types.String `tfsdk:"parent_path"`
	BlockSizeBytes types.Int64  `tfsdk:"block_size_bytes"`
}

func NewVHDDataSource() datasource.DataSource {
	return &vhdDataSource{}
}

func (d *vhdDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vhd"
}

func (d *vhdDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retrieves information about a Hyper-V virtual hard disk (VHD/VHDX).",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Required:    true,
				Description: "The file path of the virtual hard disk.",
			},
			"size_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "The size of the virtual hard disk in bytes.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The type of the virtual hard disk: Dynamic, Fixed, or Differencing.",
			},
			"parent_path": schema.StringAttribute{
				Computed:    true,
				Description: "The parent VHD path for differencing disks.",
			},
			"block_size_bytes": schema.Int64Attribute{
				Computed:    true,
				Description: "The block size of the virtual hard disk in bytes.",
			},
		},
	}
}

func (d *vhdDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vhdDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vhdDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vhd, err := d.client.GetVHD(ctx, config.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VHD",
			fmt.Sprintf("Could not read VHD %q: %s", config.Path.ValueString(), err.Error()),
		)
		return
	}

	config.Path = types.StringValue(vhd.Path)
	config.SizeBytes = types.Int64Value(vhd.Size)
	config.BlockSizeBytes = types.Int64Value(vhd.BlockSize)

	// Map VhdType int to string
	switch vhd.VhdType {
	case 2:
		config.Type = types.StringValue("Fixed")
	case 3:
		config.Type = types.StringValue("Dynamic")
	case 4:
		config.Type = types.StringValue("Differencing")
	default:
		config.Type = types.StringValue("Dynamic")
	}

	if vhd.ParentPath != "" {
		config.ParentPath = types.StringValue(vhd.ParentPath)
	} else {
		config.ParentPath = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
