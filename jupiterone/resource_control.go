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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type ControlModel struct {
	Id               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	ResourceGroupId  types.String `tfsdk:"resource_group_id"`
	State            types.String `tfsdk:"state"`
	Identifier       types.String `tfsdk:"identifier"`
	Catalog          types.String `tfsdk:"catalog"`
	Owner            types.String `tfsdk:"owner"`
	Remediation      types.String `tfsdk:"remediation"`
	ExceptionProcess types.String `tfsdk:"exception_process"`
	RequirementIds   types.List   `tfsdk:"requirement_ids"`
}

var _ resource.Resource = &ControlResource{}
var _ resource.ResourceWithConfigure = &ControlResource{}
var _ resource.ResourceWithImportState = &ControlResource{}

type ControlResource struct {
	version string
	qlient  graphql.Client
}

func NewControlResource() resource.Resource {
	return &ControlResource{}
}

// Metadata implements resource.Resource
func (*ControlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control"
}

// Configure implements resource.ResourceWithConfigure
func (r *ControlResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ControlResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom control.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the control",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the control",
			},
			"resource_group_id": schema.StringAttribute{
				Optional:    true,
				Description: "The resource group ID to scope the control to",
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The lifecycle state of the control. Must be DRAFT or LIVE on creation; can be transitioned to REVIEW or RETIRED via update.",
				Validators: []validator.String{
					stringvalidator.OneOf("DRAFT", "LIVE", "REVIEW", "RETIRED"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"identifier": schema.StringAttribute{
				Optional:    true,
				Description: "An identifier for the control used for sorting (e.g. IDAM3.5)",
			},
			"catalog": schema.StringAttribute{
				Optional:    true,
				Description: "The catalog this control belongs to (e.g. CIS Controls v8)",
			},
			"owner": schema.StringAttribute{
				Optional:    true,
				Description: "The owner of the control, represented by a user email",
			},
			"remediation": schema.StringAttribute{
				Optional:    true,
				Description: "Remediation steps in markdown format",
			},
			"exception_process": schema.StringAttribute{
				Optional:    true,
				Description: "Exception process in markdown format",
			},
			"requirement_ids": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of control framework requirement IDs to associate with this control",
				Default: listdefault.StaticValue(
					types.ListValueMust(types.StringType, []attr.Value{}),
				),
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ControlResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	desiredState := data.State.ValueString()

	var initialState client.InitialControlState
	switch desiredState {
	case "DRAFT":
		initialState = client.InitialControlStateDraft
	default:
		initialState = client.InitialControlStateLive
	}

	requirementIds := make([]string, 0)
	if !data.RequirementIds.IsNull() && !data.RequirementIds.IsUnknown() {
		resp.Diagnostics.Append(data.RequirementIds.ElementsAs(ctx, &requirementIds, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	created, err := client.CreateControl(ctx, r.qlient, client.CreateControlInput{
		Name:             data.Name.ValueString(),
		Description:      data.Description.ValueString(),
		ResourceGroupId:  data.ResourceGroupId.ValueString(),
		State:            initialState,
		Identifier:       data.Identifier.ValueString(),
		Catalog:          data.Catalog.ValueString(),
		Owner:            data.Owner.ValueString(),
		Remediation:      data.Remediation.ValueString(),
		ExceptionProcess: data.ExceptionProcess.ValueString(),
		RequirementIds:   requirementIds,
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create control", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateControl.Id)
	data.State = types.StringValue(string(created.CreateControl.State))

	tflog.Trace(ctx, "Created control",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ControlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ControlModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteControl(ctx, r.qlient, client.DeleteControlInput{Id: data.Id.ValueString()}); err != nil {
		resp.Diagnostics.AddError("failed to delete control", err.Error())
	}
}

// Read implements resource.Resource
func (r *ControlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ControlModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var c client.GetControlByIdControl
	if result, err := client.GetControlById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to find control", err.Error())
		}
		return
	} else {
		c = result.Control
	}

	data.Name = types.StringValue(c.Name)
	if c.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(c.Description)
	}
	if c.ResourceGroupId != "" || !data.ResourceGroupId.IsNull() {
		data.ResourceGroupId = types.StringValue(c.ResourceGroupId)
	}
	data.State = types.StringValue(string(c.State))
	if c.Identifier != "" || !data.Identifier.IsNull() {
		data.Identifier = types.StringValue(c.Identifier)
	}
	if c.Catalog != "" || !data.Catalog.IsNull() {
		data.Catalog = types.StringValue(c.Catalog)
	}
	if c.Owner != "" || !data.Owner.IsNull() {
		data.Owner = types.StringValue(c.Owner)
	}
	if c.Remediation != "" || !data.Remediation.IsNull() {
		data.Remediation = types.StringValue(c.Remediation)
	}
	if c.ExceptionProcess != "" || !data.ExceptionProcess.IsNull() {
		data.ExceptionProcess = types.StringValue(c.ExceptionProcess)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ControlModel
	var state ControlModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	requirementIds := make([]string, 0)
	if !data.RequirementIds.IsNull() && !data.RequirementIds.IsUnknown() {
		resp.Diagnostics.Append(data.RequirementIds.ElementsAs(ctx, &requirementIds, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	_, err := client.UpdateControl(ctx, r.qlient, client.UpdateControlInput{
		Id:               data.Id.ValueString(),
		Name:             data.Name.ValueString(),
		Description:      data.Description.ValueString(),
		ResourceGroupId:  data.ResourceGroupId.ValueString(),
		Identifier:       data.Identifier.ValueString(),
		Catalog:          data.Catalog.ValueString(),
		Owner:            data.Owner.ValueString(),
		Remediation:      data.Remediation.ValueString(),
		ExceptionProcess: data.ExceptionProcess.ValueString(),
		RequirementIds:   client.ListUpdateInput{Set: requirementIds},
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update control", err.Error())
		return
	}

	// State transitions require a separate API call.
	desiredState := data.State.ValueString()
	currentState := state.State.ValueString()
	if desiredState != "" && desiredState != currentState {
		transitioned, err := client.TransitionControlState(ctx, r.qlient, client.TransitionControlStateInput{
			ControlId:   data.Id.ValueString(),
			TargetState: client.ControlState(desiredState),
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to transition control state", err.Error())
			return
		}
		data.State = types.StringValue(string(transitioned.TransitionControlState.State))
	}

	tflog.Trace(ctx, "Updated control",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
