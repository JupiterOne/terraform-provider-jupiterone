package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// controlDataSourceItemModel is the observed state of a single control. It is
// shared by the jupiterone_control and jupiterone_controls data sources so the
// two cannot drift apart.
type controlDataSourceItemModel struct {
	Id                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	Source              types.String `tfsdk:"source"`
	SourceId            types.String `tfsdk:"source_id"`
	ResourceGroupId     types.String `tfsdk:"resource_group_id"`
	Status              types.String `tfsdk:"status"`
	LastEvaluatedOn     types.String `tfsdk:"last_evaluated_on"`
	Configured          types.Bool   `tfsdk:"configured"`
	FrameworkIds        types.List   `tfsdk:"framework_ids"`
	State               types.String `tfsdk:"state"`
	Identifier          types.String `tfsdk:"identifier"`
	Catalog             types.String `tfsdk:"catalog"`
	Owner               types.String `tfsdk:"owner"`
	Remediation         types.String `tfsdk:"remediation"`
	ExceptionProcess    types.String `tfsdk:"exception_process"`
	MitreTechnique      types.String `tfsdk:"mitre_technique"`
	CreatedOn           types.String `tfsdk:"created_on"`
	NumberOfTests       types.Int64  `tfsdk:"number_of_tests"`
	EffectiveStatus     types.String `tfsdk:"effective_status"`
	HasValidAttestation types.Bool   `tfsdk:"has_valid_attestation"`
}

// controlItemAttributes describes a control as read-only observed state.
func controlItemAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the control.",
		},
		"name": schema.StringAttribute{
			Computed:    true,
			Description: "The name of the control.",
		},
		"description": schema.StringAttribute{
			Computed:    true,
			Description: "Description of the control.",
		},
		"source": schema.StringAttribute{
			Computed:    true,
			Description: "Where the control came from: J1, UCF, UPLOAD or JSON.",
		},
		"source_id": schema.StringAttribute{
			Computed:    true,
			Description: "The identifier of the control in its source.",
		},
		"resource_group_id": schema.StringAttribute{
			Computed:    true,
			Description: "The resource group the control is scoped to.",
		},
		"status": schema.StringAttribute{
			Computed: true,
			Description: "The evaluated compliance status, PASS or FAIL. Only populated for LIVE controls. " +
				"A control covered solely by a valid attestation reads as PASS; attestation coverage is " +
				"reported separately by has_valid_attestation.",
		},
		"last_evaluated_on": schema.StringAttribute{
			Computed:    true,
			Description: "When the control was last evaluated.",
		},
		"configured": schema.BoolAttribute{
			Computed:    true,
			Description: "Whether the control is configured.",
		},
		"framework_ids": schema.ListAttribute{
			Computed:    true,
			ElementType: types.StringType,
			Description: "The frameworks this control belongs to.",
		},
		"state": schema.StringAttribute{
			Computed:    true,
			Description: "The lifecycle state of the control: DRAFT, REVIEW, LIVE or RETIRED.",
		},
		"identifier": schema.StringAttribute{
			Computed:    true,
			Description: "The identifier of the control, e.g. IDAM3.5.",
		},
		"catalog": schema.StringAttribute{
			Computed:    true,
			Description: "The catalog the control belongs to, e.g. CIS Controls v8.",
		},
		"owner": schema.StringAttribute{
			Computed:    true,
			Description: "The owner of the control, represented by a user email.",
		},
		"remediation": schema.StringAttribute{
			Computed:    true,
			Description: "Remediation steps in markdown format.",
		},
		"exception_process": schema.StringAttribute{
			Computed:    true,
			Description: "Exception process in markdown format.",
		},
		"mitre_technique": schema.StringAttribute{
			Computed:    true,
			Description: "The MITRE ATT&CK technique ID this control relates to.",
		},
		"created_on": schema.StringAttribute{
			Computed:    true,
			Description: "When the control was created.",
		},
		"number_of_tests": schema.Int64Attribute{
			Computed:    true,
			Description: "How many control tests the control has.",
		},
		"effective_status": schema.StringAttribute{
			Computed: true,
			Description: "Whether the control is producing usable results: NO_TESTS, NO_DATAPOINTS or EFFECTIVE. " +
				"NO_TESTS means no control test is defined; NO_DATAPOINTS means the tests returned no data.",
		},
		"has_valid_attestation": schema.BoolAttribute{
			Computed: true,
			Description: "Whether the control holds at least one valid, unexpired and unrevoked attestation. " +
				"Independent of status: a failing control test is never masked by an attestation.",
		},
	}
}

func controlItemObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{Attributes: controlItemAttributes()}
}

// newControlItemModel converts the observed state of a control into its
// Terraform representation. It accepts the fields common to the detail and list
// queries, which select the same set.
func newControlItemModel(
	ctx context.Context,
	id, name, description string,
	source client.ControlSource,
	sourceId, resourceGroupId string,
	status client.ControlStatus,
	lastEvaluatedOn string,
	configured bool,
	frameworkIds []string,
	state client.ControlState,
	identifier, catalog, owner, remediation, exceptionProcess, mitreTechnique, createdOn string,
	numberOfTests int,
	effectiveStatus client.ControlEffectiveness,
	hasValidAttestation bool,
) (controlDataSourceItemModel, diag.Diagnostics) {
	frameworks, diags := types.ListValueFrom(ctx, types.StringType, frameworkIds)

	return controlDataSourceItemModel{
		Id:                  types.StringValue(id),
		Name:                types.StringValue(name),
		Description:         stringOrNull(description),
		Source:              stringOrNull(string(source)),
		SourceId:            stringOrNull(sourceId),
		ResourceGroupId:     stringOrNull(resourceGroupId),
		Status:              stringOrNull(string(status)),
		LastEvaluatedOn:     stringOrNull(lastEvaluatedOn),
		Configured:          types.BoolValue(configured),
		FrameworkIds:        frameworks,
		State:               stringOrNull(string(state)),
		Identifier:          stringOrNull(identifier),
		Catalog:             stringOrNull(catalog),
		Owner:               stringOrNull(owner),
		Remediation:         stringOrNull(remediation),
		ExceptionProcess:    stringOrNull(exceptionProcess),
		MitreTechnique:      stringOrNull(mitreTechnique),
		CreatedOn:           stringOrNull(createdOn),
		NumberOfTests:       types.Int64Value(int64(numberOfTests)),
		EffectiveStatus:     stringOrNull(string(effectiveStatus)),
		HasValidAttestation: types.BoolValue(hasValidAttestation),
	}, diags
}

// --- jupiterone_control ---

func NewControlDataSource() datasource.DataSource {
	return &controlDataSource{}
}

type controlDataSource struct {
	version string
	qlient  graphql.Client
}

func (*controlDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control"
}

func (*controlDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := controlItemAttributes()

	// Either key may be used to look the control up.
	attributes["id"] = schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: "The ID of the control. Exactly one of id or source_id must be set.",
		Validators: []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("source_id")),
		},
	}
	attributes["source_id"] = schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: "The identifier of the control in its source. Exactly one of id or source_id must be set.",
	}

	resp.Schema = schema.Schema{
		Description: "The current state of a single CCM control, including its evaluated compliance status. " +
			"Managed control resources carry only declarative configuration, so evaluation results are read here.",
		Attributes: attributes,
	}
}

func (d *controlDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data controlDataSourceItemModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := client.GetControlDetail(ctx, d.qlient, data.Id.ValueString(), data.SourceId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get control", err.Error())
		return
	}

	c := result.Control
	model, diags := newControlItemModel(ctx,
		c.Id, c.Name, c.Description, c.Source, c.SourceId, c.ResourceGroupId, c.Status,
		c.LastEvaluatedOn, c.Configured, c.FrameworkIds, c.State, c.Identifier, c.Catalog,
		c.Owner, c.Remediation, c.ExceptionProcess, c.MitreTechnique, c.CreatedOn,
		c.NumberOfTests, c.EffectiveStatus, c.HasValidAttestation)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (d *controlDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// --- jupiterone_controls ---

type controlsDataSourceModel struct {
	SearchText          types.String `tfsdk:"search_text"`
	Status              types.List   `tfsdk:"status"`
	States              types.List   `tfsdk:"states"`
	Catalogs            types.List   `tfsdk:"catalogs"`
	FrameworkId         types.List   `tfsdk:"framework_id"`
	Owner               types.String `tfsdk:"owner"`
	HasValidAttestation types.Bool   `tfsdk:"has_valid_attestation"`
	MitreTechniques     types.List   `tfsdk:"mitre_techniques"`
	Effectiveness       types.String `tfsdk:"effectiveness"`
	Ou                  types.List   `tfsdk:"ou"`
	Controls            types.List   `tfsdk:"controls"`
}

func NewControlsDataSource() datasource.DataSource {
	return &controlsDataSource{}
}

type controlsDataSource struct {
	version string
	qlient  graphql.Client
}

func (*controlsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_controls"
}

func (*controlsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "CCM controls matching a filter, with their evaluated compliance status. " +
			"All filters are optional; with none set, every control is returned.",
		Attributes: map[string]schema.Attribute{
			"search_text": schema.StringAttribute{
				Optional:    true,
				Description: "Case insensitive search across name, description, identifier and id.",
			},
			"status": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Return controls whose status is any of PASS or FAIL.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("PASS", "FAIL")),
				},
			},
			"states": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Return controls in any of the given lifecycle states: DRAFT, REVIEW, LIVE or RETIRED.",
				Validators: []validator.List{
					listvalidator.ValueStringsAre(stringvalidator.OneOf("DRAFT", "REVIEW", "LIVE", "RETIRED")),
				},
			},
			"catalogs": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Return controls belonging to any of the given catalogs.",
			},
			"framework_id": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Return controls belonging to any of the given frameworks.",
			},
			"owner": schema.StringAttribute{
				Optional:    true,
				Description: "Return controls with this owner, matched exactly against a user email.",
			},
			"has_valid_attestation": schema.BoolAttribute{
				Optional: true,
				Description: "Return only controls that do, or do not, hold a valid attestation. " +
					"Independent of status.",
			},
			"mitre_techniques": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Return controls tagged with any of the given MITRE ATT&CK techniques. " +
					"Matching is by technique family, so T1059 also returns controls tagged T1059.001, and vice versa.",
			},
			"effectiveness": schema.StringAttribute{
				Optional: true,
				Description: "Return controls with this effectiveness. Only NO_TESTS and NO_DATAPOINTS are " +
					"filterable; EFFECTIVE is not supported as a filter by the API.",
				Validators: []validator.String{
					stringvalidator.OneOf("NO_TESTS", "NO_DATAPOINTS"),
				},
			},
			"ou": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Return controls related to entities in any of the given organisational units.",
			},
			"controls": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The matching controls.",
				NestedObject: controlItemObject(),
			},
		},
	}
}

func (d *controlsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data controlsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filter := client.ControlFilterInput{
		SearchText:          data.SearchText.ValueString(),
		Owner:               data.Owner.ValueString(),
		HasValidAttestation: optionalBool(data.HasValidAttestation),
	}

	if !data.Effectiveness.IsNull() {
		filter.Effectiveness = client.ControlEffectivenessFilter(data.Effectiveness.ValueString())
	}

	statuses, diags := stringSlice(ctx, data.Status)
	resp.Diagnostics.Append(diags...)
	for _, s := range statuses {
		filter.Status = append(filter.Status, client.ControlStatus(s))
	}

	states, diags := stringSlice(ctx, data.States)
	resp.Diagnostics.Append(diags...)
	for _, s := range states {
		filter.States = append(filter.States, client.ControlState(s))
	}

	filter.Catalogs, diags = stringSlice(ctx, data.Catalogs)
	resp.Diagnostics.Append(diags...)
	filter.FrameworkId, diags = stringSlice(ctx, data.FrameworkId)
	resp.Diagnostics.Append(diags...)
	filter.MitreTechniques, diags = stringSlice(ctx, data.MitreTechniques)
	resp.Diagnostics.Append(diags...)
	filter.Ou, diags = stringSlice(ctx, data.Ou)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Page through the whole result set. Returning only the first page would
	// silently under-report, which is worse than being slow.
	models := make([]controlDataSourceItemModel, 0)
	cursor := ""
	for {
		result, err := client.ListControls(ctx, d.qlient, client.ControlsQueryInput{
			Cursor: cursor,
			Filter: filter,
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to list controls", err.Error())
			return
		}

		for _, c := range result.Controls.Items {
			model, diags := newControlItemModel(ctx,
				c.Id, c.Name, c.Description, c.Source, c.SourceId, c.ResourceGroupId, c.Status,
				c.LastEvaluatedOn, c.Configured, c.FrameworkIds, c.State, c.Identifier, c.Catalog,
				c.Owner, c.Remediation, c.ExceptionProcess, c.MitreTechnique, c.CreatedOn,
				c.NumberOfTests, c.EffectiveStatus, c.HasValidAttestation)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			models = append(models, model)
		}

		if !result.Controls.PageInfo.HasNextPage || result.Controls.PageInfo.EndCursor == "" {
			break
		}
		cursor = result.Controls.PageInfo.EndCursor
	}

	controls, diags := types.ListValueFrom(ctx, controlItemObject().Type(), models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Controls = controls

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *controlsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
