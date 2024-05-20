package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// NewUserGroupDataSource is a helper function to simplify the provider implementation.
func NewUserGroupDataSource() datasource.DataSource {
	return &userGroupDataSource{}
}

// userGroupDataSource is the data source implementation.
type userGroupDataSource struct {
	version string
	qlient  graphql.Client
}

// Metadata implements resource.Resource
func (*userGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group"
}

// Schema implements resource.Resource
func (*userGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne User Group",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the user group",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "The description of the user group",
			},
			"permissions": schema.ListAttribute{
				Description: "A list of permissions for the user group.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"query_policy": schema.ListAttribute{
				Description: "A list of query policy statements for the user group.",
				Computed:    true,
				ElementType: types.MapType{
					ElemType: types.ListType{
						ElemType: types.StringType,
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *userGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserGroupModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	groups, err := client.GetGroupsByName(ctx, d.qlient, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get user group", err.Error())
		return
	}

	// Grab one group that has the same name as the given name
	if len(groups.IamGetGroupList.Items) == 0 {
		resp.Diagnostics.AddError("failed to get user group", "no group found with the given name")
		return
	}

	var group *client.GetGroupsByNameIamGetGroupListIamGroupPageItemsIamGroup

	for _, groupData := range groups.IamGetGroupList.Items {
		if groupData.GroupName == data.Name.ValueString() {
			group = &groupData
			break
		}
	}

	if group == nil {
		resp.Diagnostics.AddError("failed to get user group", "no group found with the exact given name")
		return
	}

	data.Id = types.StringValue(group.Id)
	data.Description = types.StringValue(group.GroupDescription)
	data.Permissions = group.GroupAbacPermission.Statement

	// Convert from []map[string]interface{} to []map[string][]string
	var queryPolicy []map[string][]string

	for _, statementData := range group.GroupQueryPolicy.Statement {
		var queryPolicyStatement = make(map[string][]string)

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

// Configure implements resource.ResourceWithConfigure
func (r *userGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
