package resources

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ryan/terraform-provider-hyperv/internal/client"
)

var (
	_ resource.Resource              = &isoResource{}
	_ resource.ResourceWithConfigure = &isoResource{}
)

type isoResource struct {
	client client.HyperVClient
}

type isoResourceModel struct {
	Path        types.String `tfsdk:"path"`
	VolumeLabel types.String `tfsdk:"volume_label"`
	Files       types.Map    `tfsdk:"files"`
	ContentHash types.String `tfsdk:"content_hash"`
}

func NewISOResource() resource.Resource {
	return &isoResource{}
}

func (r *isoResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iso"
}

func (r *isoResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates an ISO 9660 image file on the Hyper-V host. Useful for cloud-init NoCloud data sources.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				Required:    true,
				Description: "The file path on the Hyper-V host where the ISO will be created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"volume_label": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("cidata"),
				Description: "The volume label for the ISO image. Defaults to 'cidata' for cloud-init.",
			},
			"files": schema.MapAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "A map of filename to file content to include in the ISO image.",
			},
			"content_hash": schema.StringAttribute{
				Computed:    true,
				Description: "SHA256 hash of the ISO content (volume label + files) for drift detection.",
			},
		},
	}
}

func (r *isoResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *isoResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan isoResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filesMap := make(map[string]string)
	resp.Diagnostics.Append(plan.Files.ElementsAs(ctx, &filesMap, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := client.ISOOptions{
		Path:        plan.Path.ValueString(),
		VolumeLabel: plan.VolumeLabel.ValueString(),
		Files:       filesMap,
	}

	_, err := r.client.CreateISO(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating ISO", err.Error())
		return
	}

	plan.ContentHash = types.StringValue(computeContentHash(plan.VolumeLabel.ValueString(), filesMap))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *isoResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state isoResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	info, err := r.client.GetISO(ctx, state.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading ISO", err.Error())
		return
	}

	if !info.Exists {
		resp.State.RemoveResource(ctx)
		return
	}

	// Can't read ISO contents back from host — trust the hash in state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *isoResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan isoResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filesMap := make(map[string]string)
	resp.Diagnostics.Append(plan.Files.ElementsAs(ctx, &filesMap, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// CreateISO overwrites the existing file (FileMode.Create), so no need
	// to delete first. This avoids data loss if the create fails.
	opts := client.ISOOptions{
		Path:        plan.Path.ValueString(),
		VolumeLabel: plan.VolumeLabel.ValueString(),
		Files:       filesMap,
	}

	_, err := r.client.CreateISO(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating ISO", err.Error())
		return
	}

	plan.ContentHash = types.StringValue(computeContentHash(plan.VolumeLabel.ValueString(), filesMap))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *isoResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state isoResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteISO(ctx, state.Path.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting ISO", err.Error())
		return
	}
}

// computeContentHash produces a deterministic SHA256 hash over the volume label
// and sorted file entries, enabling drift detection for ISO content changes.
func computeContentHash(volumeLabel string, files map[string]string) string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	h := sha256.New()
	fmt.Fprintf(h, "volume_label:%s\n", volumeLabel)
	for _, name := range names {
		fmt.Fprintf(h, "file:%s:%s\n", name, files[name])
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
