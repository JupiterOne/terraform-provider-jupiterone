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

type ControlFrameworkRequirementModel struct {
	Id          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	FrameworkId types.String `tfsdk:"framework_id"`
	Description types.String `tfsdk:"description"`
	Identifier  types.String `tfsdk:"identifier"`
	Priority    types.String `tfsdk:"priority"`
	Section     types.String `tfsdk:"section"`
}

var _ resource.Resource = &ControlFrameworkRequirementResource{}
var _ resource.ResourceWithConfigure = &ControlFrameworkRequirementResource{}
var _ resource.ResourceWithImportState = &ControlFrameworkRequirementResource{}

type ControlFrameworkRequirementResource struct {
	version string
	qlient  graphql.Client
}

func NewControlFrameworkRequirementResource() resource.Resource {
	return &ControlFrameworkRequirementResource{}
}

// Metadata implements resource.Resource
func (*ControlFrameworkRequirementResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_framework_requirement"
}

// Configure implements resource.ResourceWithConfigure
func (r *ControlFrameworkRequirementResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ControlFrameworkRequirementResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A requirement belonging to a control framework.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				Required:    true,
				Description: "The title of the requirement",
			},
			"framework_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID of the control framework this requirement belongs to",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the requirement",
			},
			"identifier": schema.StringAttribute{
				Optional:    true,
				Description: "A unique identifier for the requirement",
			},
			"priority": schema.StringAttribute{
				Optional:    true,
				Description: "Priority of the requirement",
				Validators: []validator.String{
					stringvalidator.OneOf("CRITICAL", "HIGH", "MEDIUM", "LOW"),
				},
			},
			"section": schema.StringAttribute{
				Optional:    true,
				Description: "The section this requirement belongs to",
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ControlFrameworkRequirementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ControlFrameworkRequirementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlFrameworkRequirementModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateRequirement(ctx, r.qlient, client.CreateRequirementInput{
		Title:       data.Title.ValueString(),
		FrameworkId: data.FrameworkId.ValueString(),
		Description: data.Description.ValueString(),
		Identifier:  data.Identifier.ValueString(),
		Priority:    data.Priority.ValueString(),
		Section:     data.Section.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create control framework requirement", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateRequirement.Id)

	tflog.Trace(ctx, "Created control framework requirement",
		map[string]interface{}{"title": data.Title, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ControlFrameworkRequirementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ControlFrameworkRequirementModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteRequirement(ctx, r.qlient, client.DeleteRequirementInput{Id: data.Id.ValueString()}); err != nil {
		resp.Diagnostics.AddError("failed to delete control framework requirement", err.Error())
	}
}

// Read implements resource.Resource
func (r *ControlFrameworkRequirementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ControlFrameworkRequirementModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var item client.GetRequirementByIdRequirement
	if result, err := client.GetRequirementById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		if strings.Contains(err.Error(), "Could not find") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to find control framework requirement", err.Error())
		}
		return
	} else {
		item = result.Requirement
	}

	data.Title = types.StringValue(item.Title)
	if len(item.FrameworkIds) > 0 {
		data.FrameworkId = types.StringValue(item.FrameworkIds[0])
	}
	if item.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(item.Description)
	}
	if item.Identifier != "" || !data.Identifier.IsNull() {
		data.Identifier = types.StringValue(item.Identifier)
	}
	if item.Priority != "" || !data.Priority.IsNull() {
		data.Priority = types.StringValue(item.Priority)
	}
	if item.Section != "" || !data.Section.IsNull() {
		data.Section = types.StringValue(item.Section)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ControlFrameworkRequirementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ControlFrameworkRequirementModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateRequirement(ctx, r.qlient, client.UpdateRequirementInput{
		Id:          data.Id.ValueString(),
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Identifier:  data.Identifier.ValueString(),
		Priority:    data.Priority.ValueString(),
		Section:     data.Section.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update control framework requirement", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated control framework requirement",
		map[string]interface{}{"title": data.Title, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
