package jupiterone

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	Query       types.String `tfsdk:"query"`
	ResultsAre  types.String `tfsdk:"results_are"`
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

// Schema implements resource.Resource
func (*ControlTestResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A control test containing a J1QL query that evaluates control compliance.",
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
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ControlTestResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (data *ControlTestResourceModel) toQueryInput() []client.ControlTestQueryInput {
	return []client.ControlTestQueryInput{
		{
			Name:        data.Name.ValueString(),
			Query:       data.Query.ValueString(),
			ResultsAre:  client.ControlTestQueryResultsAre(data.ResultsAre.ValueString()),
			Description: data.Description.ValueString(),
		},
	}
}

// Create implements resource.Resource
func (r *ControlTestResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlTestResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateControlTest(ctx, r.qlient, client.CreateControlTestInput{
		Name:        data.Name.ValueString(),
		ControlId:   data.ControlId.ValueString(),
		Description: data.Description.ValueString(),
		Queries:     data.toQueryInput(),
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

	if len(ct.Queries) > 0 {
		q := ct.Queries[0]
		data.Query = types.StringValue(q.Query)
		data.ResultsAre = types.StringValue(string(q.ResultsAre))
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

	_, err := client.UpdateControlTest(ctx, r.qlient, client.UpdateControlTestInput{
		Id:          data.Id.ValueString(),
		Name:        data.Name.ValueString(),
		Description: data.Description.ValueString(),
		Queries:     data.toQueryInput(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update control test", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated control test",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
