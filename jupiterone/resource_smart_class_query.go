package jupiterone

import (
	"context"
	"fmt"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type SmartClassQuery struct {
	Id           types.String `json:"id,omitempty" tfsdk:"id"`
	Query        types.String `json:"query,omitempty" tfsdk:"query"`
	SmartClassId types.String `json:"smart_class_id,omitempty" tfsdk:"smart_class_id"`
	Description  types.String `json:"description,omitempty" tfsdk:"description"`
}

func NewSmartClassQueryResource() resource.Resource {
	return &SmartClassQueryResource{}
}

type SmartClassQueryResource struct {
	version string
	qlient  graphql.Client
}

func (r *SmartClassQueryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smart_class_query"
}

func (r *SmartClassQueryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A smart class query is a J1QL query that finds entities to associate with a smart class",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"query": schema.StringAttribute{
				Required:    true,
				Description: "The J1QL query to find entities for the smart class",
			},
			"smart_class_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the smart class to associate the query with",
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "A description of the smart class query",
			},
		},
	}
}

func (r *SmartClassQueryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SmartClassQueryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SmartClassQueryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SmartClassQuery

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mutationResult, err := client.CreateSmartClassQuery(ctx, r.qlient, client.CreateSmartClassQueryInput{
		Query:        data.Query.ValueString(),
		SmartClassId: data.SmartClassId.ValueString(),
		Description:  data.Description.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create smart class query", err.Error())
		return
	}

	data.Id = types.StringValue(mutationResult.CreateSmartClassQuery.Id)

	tflog.Trace(ctx, "Created smart class query", map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SmartClassQueryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SmartClassQuery

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	smartClassQuery, err := client.GetSmartClassQuery(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Failed to get smart class query", err.Error())
		}
		return
	}

	data.Id = types.StringValue(smartClassQuery.SmartClassQuery.Id)
	data.Query = types.StringValue(smartClassQuery.SmartClassQuery.Query)
	data.SmartClassId = types.StringValue(smartClassQuery.SmartClassQuery.SmartClassId)
	data.Description = types.StringValue(smartClassQuery.SmartClassQuery.Description)
}

func (r *SmartClassQueryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SmartClassQuery

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateSmartClassQuery(ctx, r.qlient, client.PatchSmartClassQueryInput{
		Id:          data.Id.ValueString(),
		Query:       data.Query.ValueString(),
		Description: data.Description.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to update smart class query", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated smart class query", map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SmartClassQueryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *SmartClassQuery

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteSmartClassQuery(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete smart class query", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}
