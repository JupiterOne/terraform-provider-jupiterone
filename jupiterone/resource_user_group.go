package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"

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

type UserGroupResource struct {
	version string
	qlient  graphql.Client
}

// UserGroupModel is the terraform HCL representation of a user group.
type UserGroupModel struct {
	Id          types.String          `json:"id,omitempty" tfsdk:"id"`
	Name        types.String          `json:"groupName,omitempty" tfsdk:"name"`
	Description types.String          `json:"groupDescription,omitempty" tfsdk:"description"`
	Permissions []string              `json:"groupAbacPermission,omitempty" tfsdk:"permissions"`
	QueryPolicy []map[string][]string `json:"groupQueryPolicy,omitempty" tfsdk:"query_policy"`
}

func NewUserGroupResource() resource.Resource {
	return &UserGroupResource{}
}

// Metadata implements resource.Resource
func (*UserGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

// Schema implements resource.Resource
func (*UserGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne User Group",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the user group",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description of the user group",
			},
			"permissions": schema.SetAttribute{
				Description: "A set of permissions for the user group.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"query_policy": schema.SetAttribute{
				Description: "A set of query policy statements for the user group.",
				Optional:    true,
				ElementType: types.MapType{
					ElemType: types.ListType{
						ElemType: types.StringType,
					},
				},
			},
		},
	}
}

// Configure implements resource.ResourceWithConfigure
func (r *UserGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Create implements resource.Resource
func (r *UserGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var queryPolicy []map[string]interface{}

	for _, statementData := range data.QueryPolicy {
		var queryPolicyStatement = make(map[string]interface{})

		for key, value := range statementData {
			queryPolicyStatement[key] = value
		}

		queryPolicy = append(queryPolicy, queryPolicyStatement)
	}

	created, err := client.CreateUserGroup(
		ctx,
		r.qlient,
		data.Name.ValueString(),
		data.Description.ValueString(),
		queryPolicy,
		data.Permissions,
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to create user group", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateIamGroup.Id)

	tflog.Trace(ctx, "Created user group",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *UserGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserGroupModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteUserGroup(ctx, r.qlient, data.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete group", err.Error())
	}
}

// Read implements resource.Resource
func (r *UserGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserGroupModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	group, err := client.GetUserGroup(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get user group", err.Error())
		return
	}

	data.Name = types.StringValue(group.IamGetGroup.GroupName)
	data.Description = types.StringValue(group.IamGetGroup.GroupDescription)
	data.Permissions = group.IamGetGroup.GroupAbacPermission.Statement

	// Convert from []map[string]interface{} to []map[string][]string
	var queryPolicy []map[string][]string

	for _, statementData := range group.IamGetGroup.GroupQueryPolicy.Statement {
		var queryPolicyStatement = make(map[string][]string) // Initialize the map

		for key, value := range statementData {
			// Was unable to parse the []string from the JSON response in any other way.
			// So we convert the value to a string and then unmarshal it into a []string.
			stringValue, stringifyError := json.Marshal(value)

			if stringifyError != nil {
				resp.Diagnostics.AddError("failed to parse query policy", stringifyError.Error())
				return
			}

			var arrayValue []string
			parseError := json.Unmarshal(stringValue, &arrayValue)

			if parseError != nil {
				resp.Diagnostics.AddError("failed to parse query policy", parseError.Error())
				return
			}

			queryPolicyStatement[key] = arrayValue
		}

		queryPolicy = append(queryPolicy, queryPolicyStatement)
	}

	data.QueryPolicy = queryPolicy

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState
func (*UserGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Update implements resource.Resource
func (r *UserGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data UserGroupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert from  []map[string][]string to []map[string]interface{}
	var queryPolicy []map[string]interface{}

	for _, statementData := range data.QueryPolicy {
		var queryPolicyStatement = make(map[string]interface{}) // Initialize the map

		for key, value := range statementData {
			queryPolicyStatement[key] = value
		}

		queryPolicy = append(queryPolicy, queryPolicyStatement)
	}

	_, err := client.UpdateUserGroup(
		ctx,
		r.qlient,
		data.Id.ValueString(),
		data.Name.ValueString(),
		data.Description.ValueString(),
		queryPolicy,
		data.Permissions,
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to update user group", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated user group",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
