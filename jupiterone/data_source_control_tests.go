package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// controlTestQueryStateModel is one query of a control test, including the
// per-query evaluation results.
type controlTestQueryStateModel struct {
	Name        types.String `tfsdk:"name"`
	Query       types.String `tfsdk:"query"`
	ResultsAre  types.String `tfsdk:"results_are"`
	RecordCount types.Int64  `tfsdk:"record_count"`
	Status      types.String `tfsdk:"status"`
	Effective   types.Bool   `tfsdk:"effective"`
}

type controlTestDataSourceItemModel struct {
	Id               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	ControlId        types.String `tfsdk:"control_id"`
	ResourceGroupId  types.String `tfsdk:"resource_group_id"`
	Status           types.String `tfsdk:"status"`
	LastEvaluatedOn  types.Int64  `tfsdk:"last_evaluated_on"`
	ReferencedRuleId types.String `tfsdk:"referenced_rule_id"`
	Queries          types.List   `tfsdk:"queries"`
}

func controlTestQueryObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the query.",
			},
			"query": schema.StringAttribute{
				Computed:    true,
				Description: "The J1QL query.",
			},
			"results_are": schema.StringAttribute{
				Computed:    true,
				Description: "Whether results indicate GOOD or BAD compliance.",
			},
			"record_count": schema.Int64Attribute{
				Computed:    true,
				Description: "How many records the query returned when it was last evaluated.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "The evaluated status of this query, PASS or FAIL.",
			},
			"effective": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the query returned usable data when it was last evaluated.",
			},
		},
	}
}

func controlTestItemAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the control test.",
		},
		"name": schema.StringAttribute{
			Computed:    true,
			Description: "The name of the control test.",
		},
		"description": schema.StringAttribute{
			Computed:    true,
			Description: "Description of the control test.",
		},
		"control_id": schema.StringAttribute{
			Computed:    true,
			Description: "The control this test belongs to.",
		},
		"resource_group_id": schema.StringAttribute{
			Computed:    true,
			Description: "The resource group the control test is scoped to.",
		},
		"status": schema.StringAttribute{
			Computed:    true,
			Description: "The evaluated status of the control test, PASS or FAIL.",
		},
		"last_evaluated_on": schema.Int64Attribute{
			Computed:    true,
			Description: "When the control test was last evaluated, as epoch milliseconds.",
		},
		"referenced_rule_id": schema.StringAttribute{
			Computed:    true,
			Description: "The ID of the rule that evaluates this control test.",
		},
		"queries": schema.ListNestedAttribute{
			Computed:     true,
			Description:  "The queries that make up this control test, with their evaluation results.",
			NestedObject: controlTestQueryObject(),
		},
	}
}

func controlTestItemObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{Attributes: controlTestItemAttributes()}
}

type controlTestQuerySource struct {
	Name        string
	Query       string
	ResultsAre  client.ControlTestQueryResultsAre
	RecordCount int
	Status      client.ControlTestStatus
	Effective   bool
}

func newControlTestItemModel(
	ctx context.Context,
	id, name, description, controlId, resourceGroupId string,
	status client.ControlTestStatus,
	lastEvaluatedOn int64,
	referencedRuleId string,
	queries []controlTestQuerySource,
) (controlTestDataSourceItemModel, diag.Diagnostics) {
	models := make([]controlTestQueryStateModel, 0, len(queries))
	for _, q := range queries {
		models = append(models, controlTestQueryStateModel{
			Name:        types.StringValue(q.Name),
			Query:       types.StringValue(q.Query),
			ResultsAre:  stringOrNull(string(q.ResultsAre)),
			RecordCount: types.Int64Value(int64(q.RecordCount)),
			Status:      stringOrNull(string(q.Status)),
			Effective:   types.BoolValue(q.Effective),
		})
	}

	queryList, diags := types.ListValueFrom(ctx, controlTestQueryObject().Type(), models)

	lastEvaluated := types.Int64Null()
	if lastEvaluatedOn != 0 {
		lastEvaluated = types.Int64Value(lastEvaluatedOn)
	}

	return controlTestDataSourceItemModel{
		Id:               types.StringValue(id),
		Name:             types.StringValue(name),
		Description:      stringOrNull(description),
		ControlId:        stringOrNull(controlId),
		ResourceGroupId:  stringOrNull(resourceGroupId),
		Status:           stringOrNull(string(status)),
		LastEvaluatedOn:  lastEvaluated,
		ReferencedRuleId: stringOrNull(referencedRuleId),
		Queries:          queryList,
	}, diags
}

// --- jupiterone_control_test ---

func NewControlTestDataSource() datasource.DataSource {
	return &controlTestDataSource{}
}

type controlTestDataSource struct {
	version string
	qlient  graphql.Client
}

func (*controlTestDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_test"
}

func (*controlTestDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attributes := controlTestItemAttributes()
	attributes["id"] = schema.StringAttribute{
		Required:    true,
		Description: "The ID of the control test.",
	}

	resp.Schema = schema.Schema{
		Description: "The current state of a single control test, including its evaluation results.",
		Attributes:  attributes,
	}
}

func (d *controlTestDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data controlTestDataSourceItemModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := client.GetControlTestDetail(ctx, d.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get control test", err.Error())
		return
	}

	ct := result.ControlTest
	queries := make([]controlTestQuerySource, 0, len(ct.Queries))
	for _, q := range ct.Queries {
		queries = append(queries, controlTestQuerySource{
			Name: q.Name, Query: q.Query, ResultsAre: q.ResultsAre,
			RecordCount: q.RecordCount, Status: q.Status, Effective: q.Effective,
		})
	}

	model, diags := newControlTestItemModel(ctx, ct.Id, ct.Name, ct.Description, ct.ControlId,
		ct.ResourceGroupId, ct.Status, ct.LastEvaluatedOn, ct.ReferencedRuleId, queries)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (d *controlTestDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// --- jupiterone_control_tests ---

type controlTestsDataSourceModel struct {
	Id           types.String `tfsdk:"id"`
	ControlId    types.String `tfsdk:"control_id"`
	ControlTests types.List   `tfsdk:"control_tests"`
}

func NewControlTestsDataSource() datasource.DataSource {
	return &controlTestsDataSource{}
}

type controlTestsDataSource struct {
	version string
	qlient  graphql.Client
}

func (*controlTestsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_tests"
}

func (*controlTestsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The control tests belonging to a control, including their evaluation results.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "A generated identifier for this result set.",
			},
			"control_id": schema.StringAttribute{
				Required:    true,
				Description: "The control whose tests should be returned.",
			},
			"control_tests": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The control tests belonging to the control.",
				NestedObject: controlTestItemObject(),
			},
		},
	}
}

func (d *controlTestsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data controlTestsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	models := make([]controlTestDataSourceItemModel, 0)
	cursor := ""
	for {
		result, err := client.ListControlTests(ctx, d.qlient, client.ControlTestsInput{
			Filters: client.ControlTestsFiltersInput{
				ControlId: data.ControlId.ValueString(),
			},
			Pagination: client.ControlTestsPaginationInput{
				Cursor: cursor,
			},
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to list control tests", err.Error())
			return
		}

		for _, ct := range result.ControlTests.Items {
			queries := make([]controlTestQuerySource, 0, len(ct.Queries))
			for _, q := range ct.Queries {
				queries = append(queries, controlTestQuerySource{
					Name: q.Name, Query: q.Query, ResultsAre: q.ResultsAre,
					RecordCount: q.RecordCount, Status: q.Status, Effective: q.Effective,
				})
			}

			model, diags := newControlTestItemModel(ctx, ct.Id, ct.Name, ct.Description, ct.ControlId,
				ct.ResourceGroupId, ct.Status, ct.LastEvaluatedOn, ct.ReferencedRuleId, queries)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			models = append(models, model)
		}

		if !result.ControlTests.PageInfo.HasNextPage || result.ControlTests.PageInfo.EndCursor == "" {
			break
		}
		cursor = result.ControlTests.PageInfo.EndCursor
	}

	controlTests, diags := types.ListValueFrom(ctx, controlTestItemObject().Type(), models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ControlTests = controlTests
	data.Id = types.StringValue(uuid.New().String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *controlTestsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
