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

// At the moment we can only support Account Type dashboards with account based API tokens.
// User Type dashboards require a user API token:
// https://github.com/JupiterOne/dashboard-service/blob/059f43edc997482099be37fd731c4645f556d0b5/src/api/graphql/public/serializers/dashboardSerializers.ts#L54
var DashboardTypes = []string{
	string(client.BoardTypeAccount),
}

type DashboardResource struct {
	version string
	qlient  graphql.Client
}

func NewDashboard() resource.Resource {
	return &DashboardResource{}
}

type DashboardModel struct {
	Id   types.String `json:"id,omitempty" tfsdk:"id"`
	Name types.String `json:"name,omitempty" tfsdk:"name"`
	Type types.String `json:"type,omitempty" tfsdk:"type"`
}

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

func (*DashboardResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r *DashboardResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

// Create implements resource.Resource.
func (r *DashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DashboardModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, err := data.BuildCreateInsightsDashboardInput()
	if err != nil {
		resp.Diagnostics.AddError("failed to build dashboard from configuration", err.Error())
		return
	}

	created, err := client.CreateDashboard(
		ctx,
		r.qlient,
		dashboard,
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to create dashboard entity", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateDashboard.Id)

	tflog.Trace(ctx, "Created dashboard",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

// Delete implements resource.Resource.
func (r *DashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *DashboardModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteDashboard(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete dashboard", err.Error())
	}
}

// Read implements resource.Resource.
func (r *DashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *DashboardModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, err := client.GetDashboard(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to get dashboard", err.Error())
		}
		return
	}

	data.Name = types.StringValue(dashboard.GetDashboard.Name)
	data.Id = types.StringValue(dashboard.GetDashboard.Id)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState
func (*DashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Schema implements resource.Resource.
func (*DashboardResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A JupiterOne insights dashboard.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the dashboard.",
			},
			"type": schema.StringAttribute{
				Description: "The type of the dashboard.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardTypes...),
				},
			},
		},
	}
}

// Update implements resource.Resource.
func (r *DashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *DashboardModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dashboard, err := data.BuildPatchInsightsDashboardInput()
	if err != nil {
		resp.Diagnostics.AddError("failed to build update dashboard from configuration", err.Error())
		return
	}

	_, err = client.UpdateDashboard(
		ctx,
		r.qlient,
		dashboard,
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to update dashboard", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated dashboard",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DashboardModel) BuildCreateInsightsDashboardInput() (client.CreateInsightsDashboardInput, error) {
	dashboard := client.CreateInsightsDashboardInput{
		Name: r.Name.ValueString(),
		Type: client.BoardType(r.Type.ValueString()),
	}

	return dashboard, nil
}

func (r *DashboardModel) BuildPatchInsightsDashboardInput() (client.PatchInsightsDashboardInput, error) {
	dashboard := client.PatchInsightsDashboardInput{
		Name:        r.Name.ValueString(),
		DashboardId: r.Id.ValueString(),
	}

	return dashboard, nil
}
