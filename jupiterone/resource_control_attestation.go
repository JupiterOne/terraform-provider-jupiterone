package jupiterone

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type ControlAttestationModel struct {
	Id           types.String `tfsdk:"id"`
	ControlId    types.String `tfsdk:"control_id"`
	Subject      types.String `tfsdk:"subject"`
	Description  types.String `tfsdk:"description"`
	ExpiresOn    types.String `tfsdk:"expires_on"`
	Owner        types.String `tfsdk:"owner"`
	DocumentLink types.String `tfsdk:"document_link"`
	Author       types.String `tfsdk:"author"`
}

var _ resource.Resource = &ControlAttestationResource{}
var _ resource.ResourceWithConfigure = &ControlAttestationResource{}
var _ resource.ResourceWithImportState = &ControlAttestationResource{}

type ControlAttestationResource struct {
	version string
	qlient  graphql.Client
}

func NewControlAttestationResource() resource.Resource {
	return &ControlAttestationResource{}
}

// Metadata implements resource.Resource
func (*ControlAttestationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_attestation"
}

// Configure implements resource.ResourceWithConfigure
func (r *ControlAttestationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.version = p.version
	r.qlient = p.Qlient
}

// Schema implements resource.Resource
func (*ControlAttestationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An attestation justifying a control by means other than an automated control test, " +
			"for example a signed policy document or a vendor report. A control covered by a valid " +
			"attestation reads as passing, but a failing control test is never masked by one.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"control_id": schema.StringAttribute{
				Required: true,
				Description: "The ID of the control this attestation justifies. The API offers no lookup " +
					"of an attestation by ID alone, so this value is also used to read the attestation back.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subject": schema.StringAttribute{
				Required:    true,
				Description: "What is being attested to",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the attestation",
			},
			"expires_on": schema.StringAttribute{
				Required: true,
				Description: "When the attestation expires, as an RFC3339 timestamp, e.g. 2027-01-31T00:00:00Z. " +
					"Must be in the future at the time it is applied.",
				Validators: []validator.String{
					rfc3339Validator{},
				},
			},
			"owner": schema.StringAttribute{
				Optional:    true,
				Description: "The owner of the attestation, represented by a user email",
			},
			"document_link": schema.StringAttribute{
				Optional:    true,
				Description: "A link to a supporting document. Must be an http or https URL.",
			},
			"author": schema.StringAttribute{
				Computed:    true,
				Description: "The author of the attestation, set from the credentials that created it",
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState.
//
// Because attestations cannot be looked up by ID alone, importing requires the
// owning control as well, given as "control_id:attestation_id".
func (*ControlAttestationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected an identifier of the form \"control_id:attestation_id\", got: %q. "+
				"An attestation cannot be read without knowing its control.", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("control_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// Create implements resource.Resource
func (r *ControlAttestationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlAttestationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	expiresOn, err := rfc3339ToEpochMillis(data.ExpiresOn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("invalid expires_on", err.Error())
		return
	}

	created, err := client.CreateAttestation(ctx, r.qlient, client.CreateAttestationInput{
		ControlId:    data.ControlId.ValueString(),
		Subject:      data.Subject.ValueString(),
		Description:  data.Description.ValueString(),
		ExpiresOn:    expiresOn,
		Owner:        data.Owner.ValueString(),
		DocumentLink: data.DocumentLink.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create attestation", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateAttestation.Id)
	data.Author = types.StringValue(created.CreateAttestation.Author)

	tflog.Trace(ctx, "Created attestation",
		map[string]interface{}{"subject": data.Subject, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource.
//
// There is no deleteAttestation mutation; revocation is the only removal path,
// and it is irreversible.
func (r *ControlAttestationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ControlAttestationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.RevokeAttestation(ctx, r.qlient, client.RevokeAttestationInput{
		Id: data.Id.ValueString(),
	}); err != nil {
		if isAttestationNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("failed to revoke attestation", err.Error())
	}
}

// Read implements resource.Resource.
//
// The attestation schema has no lookup by ID, so this lists the attestations
// for the owning control and matches locally. A revoked attestation is treated
// as gone, because revocation cannot be undone.
func (r *ControlAttestationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ControlAttestationModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := client.GetAttestationsByControlId(ctx, r.qlient, data.ControlId.ValueString())
	if err != nil {
		if isAttestationNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("failed to find attestation", err.Error())
		return
	}

	id := data.Id.ValueString()
	var found *client.GetAttestationsByControlIdAttestationsAttestation
	for i := range result.Attestations {
		if result.Attestations[i].Id == id {
			found = &result.Attestations[i]
			break
		}
	}

	// Absent, or revoked and therefore unrecoverable: drop it from state so it
	// is recreated rather than adopted.
	if found == nil || found.Revoked || found.State == client.AttestationStateRevoked {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Subject = types.StringValue(found.Subject)
	data.Author = types.StringValue(found.Author)

	if found.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(found.Description)
	}
	if found.Owner != "" || !data.Owner.IsNull() {
		data.Owner = types.StringValue(found.Owner)
	}
	if found.DocumentLink != "" || !data.DocumentLink.IsNull() {
		data.DocumentLink = types.StringValue(found.DocumentLink)
	}

	// Compare instants rather than strings. The API stores epoch milliseconds,
	// so a configured value carrying a UTC offset would otherwise be rewritten
	// into its Z-normalised equivalent on every read and never settle.
	if !sameInstant(data.ExpiresOn.ValueString(), found.ExpiresOn) {
		data.ExpiresOn = types.StringValue(epochMillisToRFC3339(found.ExpiresOn))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ControlAttestationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ControlAttestationModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	expiresOn, err := rfc3339ToEpochMillis(data.ExpiresOn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("invalid expires_on", err.Error())
		return
	}

	updated, err := client.UpdateAttestation(ctx, r.qlient, client.UpdateAttestationInput{
		Id:      data.Id.ValueString(),
		Subject: data.Subject.ValueString(),
		// Nil clears the field. An empty string would fail server-side
		// validation, since owner must be an email and document_link a URL.
		Description:  optionalString(data.Description),
		ExpiresOn:    expiresOn,
		Owner:        optionalString(data.Owner),
		DocumentLink: optionalString(data.DocumentLink),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update attestation", err.Error())
		return
	}

	data.Author = types.StringValue(updated.UpdateAttestation.Author)

	tflog.Trace(ctx, "Updated attestation",
		map[string]interface{}{"subject": data.Subject, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func isAttestationNotFound(err error) bool {
	return strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found")
}

// rfc3339ToEpochMillis converts a configured RFC3339 timestamp into the epoch
// milliseconds the API expects.
func rfc3339ToEpochMillis(s string) (float64, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid RFC3339 timestamp, e.g. 2027-01-31T00:00:00Z: %w", s, err)
	}

	return float64(t.UnixMilli()), nil
}

// epochMillisToRFC3339 renders epoch milliseconds as UTC RFC3339, keeping
// millisecond precision only when it carries information.
func epochMillisToRFC3339(ms float64) string {
	t := time.UnixMilli(int64(ms)).UTC()
	if t.Nanosecond() == 0 {
		return t.Format(time.RFC3339)
	}

	return t.Format("2006-01-02T15:04:05.000Z07:00")
}

// sameInstant reports whether a configured RFC3339 string and epoch
// milliseconds from the API refer to the same moment.
func sameInstant(configured string, ms float64) bool {
	if configured == "" {
		return false
	}

	t, err := time.Parse(time.RFC3339, configured)
	if err != nil {
		return false
	}

	return t.UnixMilli() == int64(ms)
}

// rfc3339Validator rejects an expires_on value that is not a valid RFC3339
// timestamp at plan time, rather than letting it fail against the API.
type rfc3339Validator struct{}

func (rfc3339Validator) Description(_ context.Context) string {
	return "must be an RFC3339 timestamp, e.g. 2027-01-31T00:00:00Z"
}

func (v rfc3339Validator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v rfc3339Validator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if _, err := time.Parse(time.RFC3339, req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid RFC3339 timestamp",
			fmt.Sprintf("%q is not a valid RFC3339 timestamp. Expected something like 2027-01-31T00:00:00Z.",
				req.ConfigValue.ValueString()),
		)
	}
}
