package jupiterone

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

type ControlTestResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	ControlId   types.String `tfsdk:"control_id"`
	Description types.String `tfsdk:"description"`

	// Deprecated single-query form, retained for compatibility with
	// configurations written before the API's multi-query support was exposed.
	Query      types.String `tfsdk:"query"`
	ResultsAre types.String `tfsdk:"results_are"`

	// Queries is the full-fidelity form, mapping directly onto the API's
	// queries: [ControlTestQueryInput!]! field.
	Queries types.List `tfsdk:"queries"`
}

// ControlTestQueryModel is one entry of the queries block.
type ControlTestQueryModel struct {
	Name        types.String `tfsdk:"name"`
	Query       types.String `tfsdk:"query"`
	ResultsAre  types.String `tfsdk:"results_are"`
	Description types.String `tfsdk:"description"`
}

var controlTestQueryAttrTypes = map[string]attr.Type{
	"name":        types.StringType,
	"query":       types.StringType,
	"results_are": types.StringType,
	"description": types.StringType,
}

var _ resource.Resource = &ControlTestResource{}
var _ resource.ResourceWithConfigure = &ControlTestResource{}
var _ resource.ResourceWithImportState = &ControlTestResource{}
var _ resource.ResourceWithValidateConfig = &ControlTestResource{}

type ControlTestResource struct {
	version string
	qlient  graphql.Client
}

func NewControlTestResource() resource.Resource {
	return &ControlTestResource{}
}

// Metadata implements resource.Resource
func (*ControlTestResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_test"
}

// Configure implements resource.ResourceWithConfigure
func (r *ControlTestResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ControlTestResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A control test containing one or more J1QL queries that evaluate control compliance.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the control test",
			},
			"control_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the control this test belongs to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the control test",
			},
			"query": schema.StringAttribute{
				Optional:           true,
				Description:        "The J1QL query to evaluate. Deprecated: use a `queries` block instead, which supports more than one query per test.",
				DeprecationMessage: "Use a `queries` block instead. `query` supports only a single query per control test.",
			},
			"results_are": schema.StringAttribute{
				Optional:           true,
				Description:        "Whether query results indicate GOOD or BAD compliance. Deprecated: use a `queries` block instead.",
				DeprecationMessage: "Use a `queries` block instead. `results_are` applies only to the deprecated single-query form.",
				Validators: []validator.String{
					stringvalidator.OneOf("GOOD", "BAD"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"queries": schema.ListNestedBlock{
				Description: "The J1QL queries that make up this control test. Repeat the block to evaluate more than one query.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the query",
						},
						"query": schema.StringAttribute{
							Required:    true,
							Description: "The J1QL query to evaluate",
						},
						"results_are": schema.StringAttribute{
							Required:    true,
							Description: "Whether query results indicate GOOD or BAD compliance",
							Validators: []validator.String{
								stringvalidator.OneOf("GOOD", "BAD"),
							},
						},
						"description": schema.StringAttribute{
							Optional:    true,
							Description: "Description of the query",
						},
					},
				},
			},
		},
	}
}

// ValidateConfig implements resource.ResourceWithValidateConfig.
//
// The deprecated single-query fields and the queries block are mutually
// exclusive, and exactly one of them must be present. This is hand-written
// rather than using resourcevalidator.ExactlyOneOf because a ListNestedBlock
// with no blocks present is an empty list rather than null, so the built-in
// validators would treat the block as always configured.
func (*ControlTestResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ControlTestResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasQueriesBlock := !data.Queries.IsNull() && !data.Queries.IsUnknown() && len(data.Queries.Elements()) > 0
	hasLegacyQuery := !data.Query.IsNull() || !data.ResultsAre.IsNull()

	if hasQueriesBlock && hasLegacyQuery {
		resp.Diagnostics.AddError(
			"Conflicting control test query configuration",
			"`query`/`results_are` and `queries` blocks cannot be combined. "+
				"Move the single query into a `queries` block and remove `query` and `results_are`.",
		)
		return
	}

	if !hasQueriesBlock && !hasLegacyQuery {
		resp.Diagnostics.AddError(
			"Missing control test query configuration",
			"A control test requires at least one query. Add a `queries` block.",
		)
		return
	}

	// The deprecated fields only describe a query when used together.
	if hasLegacyQuery && (data.Query.IsNull() || data.ResultsAre.IsNull()) {
		resp.Diagnostics.AddError(
			"Incomplete control test query configuration",
			"`query` and `results_are` must be set together. Prefer a `queries` block instead.",
		)
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ControlTestResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// usesLegacyQueryForm reports whether this resource is expressed with the
// deprecated top-level query fields rather than a queries block.
func (data *ControlTestResourceModel) usesLegacyQueryForm() bool {
	return !data.Query.IsNull() || !data.ResultsAre.IsNull()
}

// toQueryInput converts whichever configured form is in use into the API's
// query list.
func (data *ControlTestResourceModel) toQueryInput(ctx context.Context) ([]client.ControlTestQueryInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	if data.usesLegacyQueryForm() {
		// The deprecated form has no query name of its own, so the test name is
		// reused. This preserves the behaviour of earlier provider versions.
		return []client.ControlTestQueryInput{
			{
				Name:        data.Name.ValueString(),
				Query:       data.Query.ValueString(),
				ResultsAre:  client.ControlTestQueryResultsAre(data.ResultsAre.ValueString()),
				Description: data.Description.ValueString(),
			},
		}, diags
	}

	var queries []ControlTestQueryModel
	diags.Append(data.Queries.ElementsAs(ctx, &queries, false)...)
	if diags.HasError() {
		return nil, diags
	}

	input := make([]client.ControlTestQueryInput, 0, len(queries))
	for _, q := range queries {
		input = append(input, client.ControlTestQueryInput{
			Name:        q.Name.ValueString(),
			Query:       q.Query.ValueString(),
			ResultsAre:  client.ControlTestQueryResultsAre(q.ResultsAre.ValueString()),
			Description: q.Description.ValueString(),
		})
	}

	return input, diags
}

// Create implements resource.Resource
func (r *ControlTestResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlTestResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queries, diags := data.toQueryInput(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateControlTest(ctx, r.qlient, client.CreateControlTestInput{
		Name:        data.Name.ValueString(),
		ControlId:   data.ControlId.ValueString(),
		Description: data.Description.ValueString(),
		Queries:     queries,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to create control test", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateControlTest.Id)

	tflog.Trace(ctx, "Created control test",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ControlTestResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ControlTestResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteControlTest(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete control test", err.Error())
	}
}

// Read implements resource.Resource
func (r *ControlTestResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ControlTestResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ct client.GetControlTestByIdControlTest
	if result, err := client.GetControlTestById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to find control test", err.Error())
		}
		return
	} else {
		ct = result.ControlTest
	}

	data.Name = types.StringValue(ct.Name)
	data.ControlId = types.StringValue(ct.ControlId)
	if ct.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(ct.Description)
	}

	// Refresh whichever form the configuration uses, so that reading state does
	// not silently migrate a configuration from one form to the other.
	if data.usesLegacyQueryForm() {
		if len(ct.Queries) > 0 {
			q := ct.Queries[0]
			data.Query = types.StringValue(q.Query)
			data.ResultsAre = types.StringValue(string(q.ResultsAre))
		}
	} else {
		// The ControlTestQuery output type carries no description field, so a
		// per-query description can be written but never read back. Carry the
		// configured values forward positionally rather than dropping them,
		// which would otherwise show up as a permanent diff.
		priorDescriptions := make([]types.String, 0)
		if !data.Queries.IsNull() && !data.Queries.IsUnknown() {
			var prior []ControlTestQueryModel
			resp.Diagnostics.Append(data.Queries.ElementsAs(ctx, &prior, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			for _, p := range prior {
				priorDescriptions = append(priorDescriptions, p.Description)
			}
		}

		elements := make([]attr.Value, 0, len(ct.Queries))
		for i, q := range ct.Queries {
			description := types.StringNull()
			if i < len(priorDescriptions) {
				description = priorDescriptions[i]
			}

			obj, diags := types.ObjectValue(controlTestQueryAttrTypes, map[string]attr.Value{
				"name":        types.StringValue(q.Name),
				"query":       types.StringValue(q.Query),
				"results_are": types.StringValue(string(q.ResultsAre)),
				"description": description,
			})
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			elements = append(elements, obj)
		}

		queries, diags := types.ListValue(types.ObjectType{AttrTypes: controlTestQueryAttrTypes}, elements)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.Queries = queries
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ControlTestResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ControlTestResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queries, diags := data.toQueryInput(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateControlTest(ctx, r.qlient, client.UpdateControlTestInput{
		Id:          data.Id.ValueString(),
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Queries:     queries,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update control test", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated control test",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
