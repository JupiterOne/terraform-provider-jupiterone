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

type SmartClassResource struct {
	version string
	qlient  graphql.Client
}

func NewSmartClassResource() resource.Resource {
	return &SmartClassResource{}
}

type SmartClassRule struct {
	EvaluationStep      types.String `json:"evaluation_step,omitempty" tfsdk:"evaluation_step"`
	LastEvaluationEndOn types.Int64  `json:"last_evaluation_end_on,omitempty" tfsdk:"last_evaluation_end_on"`
}

type SmartClass struct {
	Id          types.String `json:"id,omitempty" tfsdk:"id"`
	Description types.String `json:"description,omitempty" tfsdk:"description"`
	TagName     types.String `json:"tag_name,omitempty" tfsdk:"tag_name"`
}

func (r *SmartClassResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SmartClass

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mutationResult, err := client.CreateSmartClass(ctx, r.qlient, client.CreateSmartClassInput{
		TagName:     data.TagName.ValueString(),
		Description: data.Description.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create smart class", err.Error())
		return
	}

	data.Id = types.StringValue(mutationResult.CreateSmartClass.Id)

	tflog.Trace(ctx, "Created smart class", map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SmartClassResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SmartClass

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	smartClass, err := client.GetSmartClass(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to get smart class", err.Error())
		}
		return
	}

	data.Id = types.StringValue(smartClass.SmartClass.Id)
	data.TagName = types.StringValue(smartClass.SmartClass.TagName)
	data.Description = types.StringValue(smartClass.SmartClass.Description)
}

func (r *SmartClassResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *SmartClass

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.UpdateSmartClass(ctx, r.qlient, client.PatchSmartClassInput{
		Id:          data.Id.ValueString(),
		Description: data.Description.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to update smart class", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated smart class", map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SmartClassResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *SmartClass

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteSmartClass(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete smart class", err.Error())
	}
}

func (r *SmartClassResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SmartClassResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smart_class"
}

func (r *SmartClassResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	p, ok := req.ProviderData.(*JupiterOneProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected JupiterOneProvider, got %T. Please report this issue to the provider developers.", req.ProviderData),
		)
	}

	r.version = p.version
	r.qlient = p.Qlient

}

func (r *SmartClassResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "JupiterOne Smart Class",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tag_name": schema.StringAttribute{
				Required:    true,
				Description: "The tag name of the smart class.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description of the smart class.",
			},
		},
	}

}
