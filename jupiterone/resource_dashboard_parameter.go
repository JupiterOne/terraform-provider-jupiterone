package jupiterone

import (
	"context"
	"fmt"
	"unicode"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &DashboardParameterResource{}
var _ resource.ResourceWithConfigure = &DashboardParameterResource{}
var _ resource.ResourceWithImportState = &DashboardParameterResource{}

type DashboardParameterResource struct {
	qlient graphql.Client
}

type DashboardParameterModel struct {
	Id                 types.String `json:"id,omitempty" tfsdk:"id"`
	DashboardId        types.String `json:"dashboard_id" tfsdk:"dashboard_id"`
	Label              types.String `json:"label" tfsdk:"label"`
	Name               types.String `json:"name" tfsdk:"name"`
	ValueType          types.String `json:"value_type" tfsdk:"value_type"`
	Options            types.List   `json:"options,omitempty" tfsdk:"options"`
	Type               types.String `json:"type" tfsdk:"type"`
	Default            types.String `json:"default,omitempty" tfsdk:"default"`
	DisableCustomInput types.Bool   `json:"disable_custom_input,omitempty" tfsdk:"disable_custom_input"`
	RequireValue       types.Bool   `json:"require_value,omitempty" tfsdk:"require_value"`
}

func NewDashboardParameterResource() resource.Resource {
	return &DashboardParameterResource{}
}

func (r *DashboardParameterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard_parameter"
}

func (r *DashboardParameterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.qlient = p.Qlient
}

func (r *DashboardParameterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *DashboardParameterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that the name is alphanumeric
	if !isAlphanumeric(data.Name.ValueString()) {
		resp.Diagnostics.AddError(
			"Invalid Name",
			"The 'name' field must contain only alphanumeric characters.",
		)
		return
	}

	var options []string
	diags := data.Options.ElementsAs(ctx, &options, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.CreateDashboardParameterInput{
		DashboardId:        data.DashboardId.ValueString(),
		Label:              data.Label.ValueString(),
		Name:               data.Name.ValueString(),
		ValueType:          client.DashboardParameterValueType(data.ValueType.ValueString()),
		Options:            options,
		Type:               client.DashboardParameterType(data.Type.ValueString()),
		Default:            data.Default.ValueString(),
		DisableCustomInput: data.DisableCustomInput.ValueBool(),
		RequireValue:       data.RequireValue.ValueBool(),
	}

	created, err := client.CreateDashboardParameter(ctx, r.qlient, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create dashboard parameter", err.Error())
		return
	}

	// Set the ID only once here
	data.Id = types.StringValue(created.CreateDashboardParameter.Id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DashboardParameterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DashboardParameterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := client.DashboardParameter(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard parameter", err.Error())
		return
	}

	// Ensure the ID is not modified unless it changes
	if data.Id.ValueString() != response.DashboardParameter.Id {
		data.Id = types.StringValue(response.DashboardParameter.Id)
	}

	// Update other fields only if they differ
	if data.DashboardId.ValueString() != response.DashboardParameter.DashboardId {
		data.DashboardId = types.StringValue(response.DashboardParameter.DashboardId)
	}
	if data.Label.ValueString() != response.DashboardParameter.Label {
		data.Label = types.StringValue(response.DashboardParameter.Label)
	}
	if data.Name.ValueString() != response.DashboardParameter.Name {
		data.Name = types.StringValue(response.DashboardParameter.Name)
	}
	if data.ValueType.ValueString() != string(response.DashboardParameter.ValueType) {
		data.ValueType = types.StringValue(string(response.DashboardParameter.ValueType))
	}
	optionsList, diags := types.ListValueFrom(ctx, types.StringType, response.DashboardParameter.Options)
	resp.Diagnostics.Append(diags...)
	if !data.Options.Equal(optionsList) {
		data.Options = optionsList
	}
	if data.Type.ValueString() != string(response.DashboardParameter.Type) {
		data.Type = types.StringValue(string(response.DashboardParameter.Type))
	}
	if data.Default.ValueString() != response.DashboardParameter.Default {
		data.Default = types.StringValue(response.DashboardParameter.Default)
	}
	if data.DisableCustomInput.ValueBool() != response.DashboardParameter.DisableCustomInput {
		data.DisableCustomInput = types.BoolValue(response.DashboardParameter.DisableCustomInput)
	}
	if data.RequireValue.ValueBool() != response.DashboardParameter.RequireValue {
		data.RequireValue = types.BoolValue(response.DashboardParameter.RequireValue)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DashboardParameterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *DashboardParameterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state DashboardParameterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Proceed with the update if the name hasn't changed
	var options []string
	diags := data.Options.ElementsAs(ctx, &options, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.PatchDashboardParameterInput{
		Id:                 data.Id.ValueString(),
		Label:              data.Label.ValueString(),
		ValueType:          client.DashboardParameterValueType(data.ValueType.ValueString()),
		Options:            options,
		Type:               client.DashboardParameterType(data.Type.ValueString()),
		Default:            data.Default.ValueString(),
		DisableCustomInput: data.DisableCustomInput.ValueBool(),
		RequireValue:       data.RequireValue.ValueBool(),
	}

	_, err := client.PatchDashboardParameter(ctx, r.qlient, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update dashboard parameter", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DashboardParameterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DashboardParameterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.DeleteDashboardParameter(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete dashboard parameter", err.Error())
	}
}

func (*DashboardParameterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A JupiterOne dashboard parameter.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier of the dashboard parameter.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dashboard_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the dashboard.",
			},
			"label": schema.StringAttribute{
				Required:    true,
				Description: "The label of the parameter.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the parameter.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value_type": schema.StringAttribute{
				Required:    true,
				Description: "The value type of the parameter.",
			},
			"options": schema.ListAttribute{
				Optional:    true,
				Description: "The options for the parameter.",
				ElementType: types.StringType,
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the parameter.",
			},
			"default": schema.StringAttribute{
				Optional:    true,
				Description: "The default value of the parameter.",
			},
			"disable_custom_input": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether custom input is disabled.",
			},
			"require_value": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether a value is required.",
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*DashboardParameterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Helper function to check if a string is alphanumeric
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
