package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type attestationStateModel struct {
	Id           types.String `tfsdk:"id"`
	ControlId    types.String `tfsdk:"control_id"`
	Subject      types.String `tfsdk:"subject"`
	Description  types.String `tfsdk:"description"`
	ExpiresOn    types.String `tfsdk:"expires_on"`
	Owner        types.String `tfsdk:"owner"`
	Author       types.String `tfsdk:"author"`
	DocumentLink types.String `tfsdk:"document_link"`
	Revoked      types.Bool   `tfsdk:"revoked"`
	CreatedOn    types.String `tfsdk:"created_on"`
	State        types.String `tfsdk:"state"`
}

type attestationsDataSourceModel struct {
	ControlId          types.String `tfsdk:"control_id"`
	Owner              types.String `tfsdk:"owner"`
	State              types.String `tfsdk:"state"`
	ExpiringWithinDays types.Int64  `tfsdk:"expiring_within_days"`
	IncludeRevoked     types.Bool   `tfsdk:"include_revoked"`
	Attestations       types.List   `tfsdk:"attestations"`
}

func attestationObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the attestation.",
			},
			"control_id": schema.StringAttribute{
				Computed:    true,
				Description: "The control this attestation justifies.",
			},
			"subject": schema.StringAttribute{
				Computed:    true,
				Description: "What is being attested to.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the attestation.",
			},
			"expires_on": schema.StringAttribute{
				Computed:    true,
				Description: "When the attestation expires, as an RFC3339 timestamp in UTC.",
			},
			"owner": schema.StringAttribute{
				Computed:    true,
				Description: "The owner of the attestation, represented by a user email.",
			},
			"author": schema.StringAttribute{
				Computed:    true,
				Description: "The author of the attestation, set from the credentials that created it.",
			},
			"document_link": schema.StringAttribute{
				Computed:    true,
				Description: "A link to a supporting document.",
			},
			"revoked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the attestation has been revoked. Revocation is irreversible.",
			},
			"created_on": schema.StringAttribute{
				Computed:    true,
				Description: "When the attestation was created.",
			},
			"state": schema.StringAttribute{
				Computed: true,
				Description: "The derived lifecycle state: ACTIVE, EXPIRED or REVOKED. Computed from " +
					"revocation and expiry rather than stored, and REVOKED takes precedence over EXPIRED.",
			},
		},
	}
}

func NewAttestationsDataSource() datasource.DataSource {
	return &attestationsDataSource{}
}

type attestationsDataSource struct {
	version string
	qlient  graphql.Client
}

func (*attestationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_attestations"
}

func (*attestationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Control attestations, filtered and sorted by expiry, soonest first. " +
			"Without a control_id filter this requires CCM read-admin permission; with one it requires " +
			"read access to that control.",
		Attributes: map[string]schema.Attribute{
			"control_id": schema.StringAttribute{
				Optional:    true,
				Description: "Return attestations for this control only.",
			},
			"owner": schema.StringAttribute{
				Optional:    true,
				Description: "Return attestations with this owner, matched against a user email.",
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Description: "Return attestations in this derived state: ACTIVE, EXPIRED or REVOKED.",
				Validators: []validator.String{
					stringvalidator.OneOf("ACTIVE", "EXPIRED", "REVOKED"),
				},
			},
			"expiring_within_days": schema.Int64Attribute{
				Optional:    true,
				Description: "Return attestations whose expiry falls within the next given number of days.",
			},
			"include_revoked": schema.BoolAttribute{
				Optional: true,
				Description: "Also return revoked attestations. Revocation is a soft delete, so revoked " +
					"attestations persist and are otherwise omitted.",
			},
			"attestations": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The matching attestations.",
				NestedObject: attestationObject(),
			},
		},
	}
}

func (d *attestationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data attestationsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.AttestationQueryInput{
		ControlId:          data.ControlId.ValueString(),
		Owner:              data.Owner.ValueString(),
		ExpiringWithinDays: int(data.ExpiringWithinDays.ValueInt64()),
		IncludeDeleted:     data.IncludeRevoked.ValueBool(),
	}

	if !data.State.IsNull() {
		input.State = client.AttestationState(data.State.ValueString())
	}

	result, err := client.ListAttestations(ctx, d.qlient, input)
	if err != nil {
		resp.Diagnostics.AddError("failed to list attestations", err.Error())
		return
	}

	models := make([]attestationStateModel, 0, len(result.Attestations))
	for _, a := range result.Attestations {
		models = append(models, attestationStateModel{
			Id:           types.StringValue(a.Id),
			ControlId:    stringOrNull(a.ControlId),
			Subject:      types.StringValue(a.Subject),
			Description:  stringOrNull(a.Description),
			ExpiresOn:    types.StringValue(epochMillisToRFC3339(a.ExpiresOn)),
			Owner:        stringOrNull(a.Owner),
			Author:       stringOrNull(a.Author),
			DocumentLink: stringOrNull(a.DocumentLink),
			Revoked:      types.BoolValue(a.Revoked),
			CreatedOn:    stringOrNull(a.CreatedOn),
			State:        stringOrNull(string(a.State)),
		})
	}

	attestations, diags := types.ListValueFrom(ctx, attestationObject().Type(), models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Attestations = attestations

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *attestationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(*JupiterOneProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected JupiterOneProvider, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.version = p.version
	d.qlient = p.Qlient
}
