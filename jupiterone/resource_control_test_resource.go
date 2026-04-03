package jupiterone

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

type ControlTestQueryModel struct {
	Name        types.String `tfsdk:"name"`
	Query       types.String `tfsdk:"query"`
	ResultsAre  types.String `tfsdk:"results_are"`
	Description types.String `tfsdk:"description"`
}

type ControlTestResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	ControlId   types.String `tfsdk:"control_id"`
	Description types.String `tfsdk:"description"`
	Queries     types.List   `tfsdk:"queries"`
}

var _ resource.Resource = &ControlTestResource{}
var _ resource.ResourceWithConfigure = &ControlTestResource{}
var _ resource.ResourceWithImportState = &ControlTestResource{}

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

var controlTestQueryAttrTypes = map[string]schema.Attribute{
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
		Description: "Description of what this query tests",
	},
}

// Schema implements resource.Resource
func (*ControlTestResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A control test containing J1QL queries that evaluate control compliance.",
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
			"queries": schema.ListNestedAttribute{
				Required:    true,
				Description: "List of J1QL queries that make up this control test",
				NestedObject: schema.NestedAttributeObject{
					Attributes: controlTestQueryAttrTypes,
				},
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ControlTestResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func controlTestQueriesToInput(ctx context.Context, queriesList types.List) ([]client.ControlTestQueryInput, error) {
	var models []ControlTestQueryModel
	if diags := queriesList.ElementsAs(ctx, &models, false); diags.HasError() {
		return nil, fmt.Errorf("failed to read queries")
	}

	inputs := make([]client.ControlTestQueryInput, len(models))
	for i, q := range models {
		inputs[i] = client.ControlTestQueryInput{
			Name:        q.Name.ValueString(),
			Query:       q.Query.ValueString(),
			ResultsAre:  client.ControlTestQueryResultsAre(q.ResultsAre.ValueString()),
			Description: q.Description.ValueString(),
		}
	}
	return inputs, nil
}

func controlTestQueryObjectAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":        types.StringType,
		"query":       types.StringType,
		"results_are": types.StringType,
		"description": types.StringType,
	}
}

// Create implements resource.Resource
func (r *ControlTestResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlTestResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queries, err := controlTestQueriesToInput(ctx, data.Queries)
	if err != nil {
		resp.Diagnostics.AddError("failed to read queries", err.Error())
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

	// The API does not return query descriptions, so we preserve them from state
	// to avoid perpetual diffs. We only refresh name, query, and results_are.
	var stateQueries []ControlTestQueryModel
	resp.Diagnostics.Append(data.Queries.ElementsAs(ctx, &stateQueries, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updatedQueries := make([]ControlTestQueryModel, len(ct.Queries))
	for i, q := range ct.Queries {
		desc := types.StringNull()
		if i < len(stateQueries) {
			desc = stateQueries[i].Description
		}
		updatedQueries[i] = ControlTestQueryModel{
			Name:        types.StringValue(q.Name),
			Query:       types.StringValue(q.Query),
			ResultsAre:  types.StringValue(string(q.ResultsAre)),
			Description: desc,
		}
	}

	queriesList, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: controlTestQueryObjectAttrTypes()}, updatedQueries)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Queries = queriesList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ControlTestResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ControlTestResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	queries, err := controlTestQueriesToInput(ctx, data.Queries)
	if err != nil {
		resp.Diagnostics.AddError("failed to read queries", err.Error())
		return
	}

	_, err = client.UpdateControlTest(ctx, r.qlient, client.UpdateControlTestInput{
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
