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

type SmartClassTag struct {
	Id           types.String `json:"id,omitempty" tfsdk:"id"`
	SmartClassId types.String `json:"smart_class_id,omitempty" tfsdk:"smart_class_id"`
	Name         types.String `json:"name,omitempty" tfsdk:"name"`
	Type         types.String `json:"type,omitempty" tfsdk:"type"`
	Value        types.String `json:"value,omitempty" tfsdk:"value"`
}

func NewSmartClassTagResource() resource.Resource {
	return &SmartClassTagResource{}
}

type SmartClassTagResource struct {
	version string
	qlient  graphql.Client
}

func (r *SmartClassTagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smart_class_tag"
}

func (r *SmartClassTagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A smart class tag is another tag applied to entities found by a smart class query",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"smart_class_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the smart class to associate the tag with",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name (key) of the tag",
			},
			"type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("string", "boolean", "number"),
				},
				Description: "The type of the tag, one of 'string', 'boolean', or 'number'",
			},
			"value": schema.StringAttribute{
				Required:    true,
				Description: "The value of the tag as a string",
			},
		},
	}
}

func (r *SmartClassTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SmartClassTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SmartClassTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SmartClassTag

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mutationResult, err := client.CreateSmartClassTag(ctx, r.qlient, client.CreateSmartClassTagInput{
		SmartClassId: data.SmartClassId.ValueString(),
		Type:         client.SmartClassTagType(data.Type.ValueString()),
		Name:         data.Name.ValueString(),
		Value:        data.Value.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to create smart class Tag", err.Error())
		return
	}

	data.Id = types.StringValue(mutationResult.CreateSmartClassTag.Id)

	tflog.Trace(ctx, "Created smart class Tag", map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SmartClassTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *SmartClassTag

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	smartClass, err := client.GetSmartClass(ctx, r.qlient, data.SmartClassId.ValueString())

	if err != nil {
		resp.Diagnostics.AddError("Failed to read smart class Tag", err.Error())
		return
	}

	var smartClassTag client.GetSmartClassSmartClassTagsSmartClassTag
	found := false
	for _, tag := range smartClass.SmartClass.Tags {
		if strings.EqualFold(tag.Id, data.Id.ValueString()) {
			smartClassTag = tag
			found = true
			break
		}
	}

	if !found {
		resp.Diagnostics.AddError("Failed to read smart class Tag", "Tag not found on smart class")
		return
	}

	data.Id = types.StringValue(smartClassTag.Id)
	data.Type = types.StringValue(string(smartClassTag.Type))
	data.Name = types.StringValue(smartClassTag.Name)
	data.Value = types.StringValue(smartClassTag.Value)
	data.SmartClassId = types.StringValue(data.SmartClassId.ValueString())
}

func (r *SmartClassTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SmartClassTag

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateSmartClassTag(ctx, r.qlient, client.PatchSmartClassTagInput{
		Id:    data.Id.ValueString(),
		Type:  client.SmartClassTagType(data.Type.ValueString()),
		Name:  data.Name.ValueString(),
		Value: data.Value.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("Failed to update smart class Tag", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated smart class Tag", map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SmartClassTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *SmartClassTag

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteSmartClassTag(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete smart class Tag", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}
