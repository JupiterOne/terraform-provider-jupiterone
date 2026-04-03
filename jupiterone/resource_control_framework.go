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

type ControlFrameworkModel struct {
	Id              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	ResourceGroupId types.String `tfsdk:"resource_group_id"`
	Owner           types.String `tfsdk:"owner"`
}

var _ resource.Resource = &ControlFrameworkResource{}
var _ resource.ResourceWithConfigure = &ControlFrameworkResource{}
var _ resource.ResourceWithImportState = &ControlFrameworkResource{}

type ControlFrameworkResource struct {
	version string
	qlient  graphql.Client
}

func NewControlFrameworkResource() resource.Resource {
	return &ControlFrameworkResource{}
}

// Metadata implements resource.Resource
func (*ControlFrameworkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_framework"
}

// Configure implements resource.ResourceWithConfigure
func (r *ControlFrameworkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (*ControlFrameworkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom control framework.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the control framework",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the control framework",
			},
			"resource_group_id": schema.StringAttribute{
				Optional:    true,
				Description: "The resource group ID to scope the framework to",
			},
			"owner": schema.StringAttribute{
				Optional:    true,
				Description: "The owner of the framework",
			},
		},
	}
}

// ImportState implements resource.ResourceWithImportState
func (*ControlFrameworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create implements resource.Resource
func (r *ControlFrameworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ControlFrameworkModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	created, err := client.CreateFramework(ctx, r.qlient, client.CreateFrameworkInput{
		Title:           data.Name.ValueString(),
		Description:     data.Description.ValueString(),
		ResourceGroupId: data.ResourceGroupId.ValueString(),
		Owner:           data.Owner.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create control framework", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateFramework.Id)

	tflog.Trace(ctx, "Created control framework",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *ControlFrameworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ControlFrameworkModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteFramework(ctx, r.qlient, client.DeleteFrameworkInput{Id: data.Id.ValueString()}); err != nil {
		resp.Diagnostics.AddError("failed to delete control framework", err.Error())
	}
}

// Read implements resource.Resource
func (r *ControlFrameworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ControlFrameworkModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var f client.GetFrameworkByIdControlFramework
	if result, err := client.GetFrameworkById(ctx, r.qlient, data.Id.ValueString()); err != nil {
		if strings.Contains(err.Error(), "Could not find") || strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to find control framework", err.Error())
		}
		return
	} else {
		f = result.ControlFramework
	}

	data.Name = types.StringValue(f.Name)
	if f.Description != "" || !data.Description.IsNull() {
		data.Description = types.StringValue(f.Description)
	}
	if f.ResourceGroupId != "" || !data.ResourceGroupId.IsNull() {
		data.ResourceGroupId = types.StringValue(f.ResourceGroupId)
	}
	if f.Owner != "" || !data.Owner.IsNull() {
		data.Owner = types.StringValue(f.Owner)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource
func (r *ControlFrameworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ControlFrameworkModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// The API validates resourceGroupId as a UUID and rejects empty strings.
	// We must send null (omitempty) rather than "" when it is not set, so we
	// use *string and only populate it when an actual value is present.
	var resourceGroupId *string
	if !data.ResourceGroupId.IsNull() && !data.ResourceGroupId.IsUnknown() && data.ResourceGroupId.ValueString() != "" {
		v := data.ResourceGroupId.ValueString()
		resourceGroupId = &v
	}

	_, err := client.UpdateFramework(ctx, r.qlient, client.UpdateFrameworkInput{
		FrameworkId:     data.Id.ValueString(),
		Name:            data.Name.ValueString(),
		Description:     data.Description.ValueString(),
		ResourceGroupId: resourceGroupId,
		Owner:           data.Owner.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to update control framework", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated control framework",
		map[string]interface{}{"name": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
