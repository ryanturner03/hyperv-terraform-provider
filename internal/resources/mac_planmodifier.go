package resources

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// dynamicMACFromStaticMAC plans dynamic_mac_address = false whenever a static
// mac_address is configured on the same network_adapter block.
//
// Hyper-V treats the two as mutually exclusive: assigning a static MAC turns
// dynamic addressing off. The attribute's schema default is true, so a config
// that sets only mac_address planned dynamic_mac_address = true while the host
// reported false after apply, producing:
//
//	Provider produced inconsistent result after apply
//	.network_adapter[0].dynamic_mac_address: was cty.True, but now cty.False
//
// Correcting this at plan time (rather than patching state afterwards) keeps the
// plan an honest prediction of the result, which is what Terraform requires. An
// explicit dynamic_mac_address in the config is always respected.
type dynamicMACFromStaticMAC struct{}

func (m dynamicMACFromStaticMAC) Description(_ context.Context) string {
	return "Plans false when a static mac_address is configured."
}

func (m dynamicMACFromStaticMAC) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m dynamicMACFromStaticMAC) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// An explicit value in the configuration always wins.
	var configVal types.Bool
	if diags := req.Config.GetAttribute(ctx, req.Path, &configVal); !diags.HasError() {
		if !configVal.IsNull() && !configVal.IsUnknown() {
			return
		}
	}

	// Sibling mac_address on the same adapter block.
	macPath := req.Path.ParentPath().AtName("mac_address")
	var mac types.String
	if diags := req.Config.GetAttribute(ctx, macPath, &mac); diags.HasError() {
		return
	}

	if !mac.IsNull() && !mac.IsUnknown() && mac.ValueString() != "" {
		resp.PlanValue = types.BoolValue(false)
	}
}
