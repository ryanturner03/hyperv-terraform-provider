package provider

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
	"github.com/ryan/terraform-provider-hyperv/internal/datasources"
	"github.com/ryan/terraform-provider-hyperv/internal/resources"
)

var _ provider.Provider = &HyperVProvider{}

type HyperVProvider struct {
	version string
}

type HyperVProviderModel struct {
	Host      types.String `tfsdk:"host"`
	Port      types.Int64  `tfsdk:"port"`
	UseTLS    types.Bool   `tfsdk:"use_tls"`
	Insecure  types.Bool   `tfsdk:"insecure"`
	Timeout   types.String `tfsdk:"timeout"`
	AuthType  types.String `tfsdk:"auth_type"`
	Username  types.String `tfsdk:"username"`
	Password  types.String `tfsdk:"password"`
	Realm     types.String `tfsdk:"realm"`
	KrbConfig types.String `tfsdk:"krb_config"`
	KrbCCache types.String `tfsdk:"krb_ccache"`
	SPN       types.String `tfsdk:"spn"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &HyperVProvider{version: version}
	}
}

func (p *HyperVProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hyperv"
	resp.Version = p.version
}

func (p *HyperVProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Hyper-V resources via WinRM.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Required:    true,
				Description: "Hyper-V host FQDN or IP address.",
			},
			"port": schema.Int64Attribute{
				Optional:    true,
				Description: "WinRM port. Default: 5986.",
			},
			"use_tls": schema.BoolAttribute{
				Optional:    true,
				Description: "Use HTTPS for WinRM. Default: true.",
			},
			"insecure": schema.BoolAttribute{
				Optional:    true,
				Description: "Skip TLS certificate verification. Default: false.",
			},
			"timeout": schema.StringAttribute{
				Optional:    true,
				Description: "WinRM operation timeout. Default: 30s.",
			},
			"auth_type": schema.StringAttribute{
				Required:    true,
				Description: "Authentication type: kerberos, ntlm, or basic.",
				Validators: []validator.String{
					stringvalidator.OneOf("kerberos", "ntlm", "basic"),
				},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username for NTLM or Basic auth.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password for NTLM or Basic auth. Can also be set via HYPERV_PASSWORD env var.",
			},
			"realm": schema.StringAttribute{
				Optional:    true,
				Description: "Kerberos realm (e.g., DOMAIN.LOCAL). Required for kerberos auth.",
			},
			"krb_config": schema.StringAttribute{
				Optional:    true,
				Description: "Path to krb5.conf. Default: /etc/krb5.conf.",
			},
			"krb_ccache": schema.StringAttribute{
				Optional:    true,
				Description: "Path to Kerberos credential cache file (from kinit). When set, ticket-based auth is used instead of username/password.",
			},
			"spn": schema.StringAttribute{
				Optional:    true,
				Description: "Kerberos Service Principal Name. Default: HTTP/<host>.",
			},
		},
	}
}

func (p *HyperVProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config HyperVProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	host := config.Host.ValueString()
	authType := config.AuthType.ValueString()

	port := int64(5986)
	if !config.Port.IsNull() {
		port = config.Port.ValueInt64()
	}
	useTLS := true
	if !config.UseTLS.IsNull() {
		useTLS = config.UseTLS.ValueBool()
	}
	insecure := false
	if !config.Insecure.IsNull() {
		insecure = config.Insecure.ValueBool()
	}
	if insecure {
		resp.Diagnostics.AddWarning("Insecure TLS", "Certificate verification is disabled. This should only be used in development environments.")
	}
	timeoutStr := "30s"
	if !config.Timeout.IsNull() {
		timeoutStr = config.Timeout.ValueString()
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		resp.Diagnostics.AddError("Invalid timeout", fmt.Sprintf("Cannot parse timeout %q: %s", timeoutStr, err))
		return
	}

	password := config.Password.ValueString()
	if password == "" {
		password = os.Getenv("HYPERV_PASSWORD")
	}

	if authType == "basic" && !useTLS {
		resp.Diagnostics.AddError("Insecure basic auth", "Basic auth requires use_tls = true to prevent sending credentials in cleartext.")
		return
	}
	if authType == "ntlm" && !useTLS {
		resp.Diagnostics.AddWarning(
			"NTLM over plaintext HTTP",
			"NTLM authentication without TLS exposes credential hashes to network interception. Set use_tls = true for production environments.",
		)
	}

	c, err := client.NewWinRMClient(client.ConnectionConfig{
		Host:      host,
		Port:      int(port),
		UseTLS:    useTLS,
		Insecure:  insecure,
		Timeout:   timeout,
		AuthType:  client.AuthType(authType),
		Username:  config.Username.ValueString(),
		Password:  password,
		Realm:     config.Realm.ValueString(),
		KrbConfig: config.KrbConfig.ValueString(),
		KrbCCache: config.KrbCCache.ValueString(),
		SPN:       config.SPN.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("WinRM connection failed", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *HyperVProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewVMResource,
		resources.NewVHDResource,
		resources.NewVirtualSwitchResource,
		resources.NewNetworkAdapterResource,
		resources.NewISOResource,
		resources.NewDVDDriveResource,
		resources.NewHardDriveResource,
	}
}

func (p *HyperVProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		datasources.NewVMDataSource,
		datasources.NewVHDDataSource,
		datasources.NewVirtualSwitchDataSource,
		datasources.NewNetworkAdapterDataSource,
		datasources.NewDVDDriveDataSource,
		datasources.NewHardDriveDataSource,
	}
}
