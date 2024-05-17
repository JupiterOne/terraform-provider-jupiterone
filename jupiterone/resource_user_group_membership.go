package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &UserGroupMembershipResource{}
var _ resource.ResourceWithConfigure = &UserGroupMembershipResource{}

type UserGroupMembershipResource struct {
	version string
	qlient  graphql.Client
}

// UserGroupMembershipModel is the terraform HCL representation of a user group membership.
type UserGroupMembershipModel struct {
	Id      types.String `json:"id,omitempty" tfsdk:"id"`
	GroupId types.String `json:"groupId,omitempty" tfsdk:"group_id"`
	Email   types.String `json:"email,omitempty" tfsdk:"email"`
}

func NewUserGroupMembershipResource() resource.Resource {
	return &UserGroupMembershipResource{}
}

// Metadata implements resource.Resource
func (*UserGroupMembershipResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_group_membership"
}

// Schema implements resource.Resource
func (*UserGroupMembershipResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne User Group Membership",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"group_id": schema.StringAttribute{
				Required: true,
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "The email of the user to add to the group",
			},
		},
	}
}

// Configure implements resource.ResourceWithConfigure
func (r *UserGroupMembershipResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *UserGroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserGroupMembershipModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.InviteUser(
		ctx,
		r.qlient,
		data.Email.ValueString(),
		data.GroupId.ValueString(),
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to create user group membership", err.Error())
		return
	}

	tflog.Trace(ctx, "Created user group membership",
		map[string]interface{}{"groupId": data.GroupId, "email": data.Email})

	data.Id = types.StringValue("placeholder")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *UserGroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserGroupMembershipModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// We need to delete the user from the group if the user is part of the group
	var usersResponse, getUserErr = client.GetUsersByEmail(ctx, r.qlient, data.Email.ValueString())

	if getUserErr != nil {
		resp.Diagnostics.AddError("failed to get user", getUserErr.Error())
		return
	}

	// User exists in this account, check for the group and remove them
	if len(usersResponse.IamGetUserList.Items) > 0 {
		var user = usersResponse.IamGetUserList.Items[0]

		// Iterate through groups and remove the user from the group
		for _, group := range user.UserGroups.Items {
			if group.Id == data.GroupId.ValueString() {
				if _, removeFromGroupErr := client.RemoveUserFromGroup(ctx, r.qlient, data.Email.ValueString(), group.Id); removeFromGroupErr != nil {
					resp.Diagnostics.AddError("failed to remove user from group", removeFromGroupErr.Error())
				}
				tflog.Trace(ctx, "User was removed from group",
					map[string]interface{}{"groupId": data.GroupId, "email": data.Email})

				// Can early return because we have taken care of the removal of this user from the group
				return
			}
		}
	}

	// If they are not yet part of the group, we should remove the invitations
	tflog.Trace(ctx, "User was not part of this group, removing any related invitations",
		map[string]interface{}{"groupId": data.GroupId, "email": data.Email})

	var invitations, getInvitesErr = client.GetInvitations(ctx, r.qlient)

	if getInvitesErr != nil {
		resp.Diagnostics.AddError("failed to get invitations", getInvitesErr.Error())
		return
	}

	for _, invite := range invitations.IamGetAccount.AccountInvitations.Items {
		if invite.Email == data.Email.ValueString() && invite.GroupId == data.GroupId.ValueString() {
			if _, removeInviteErr := client.RevokeInvitation(ctx, r.qlient, invite.Id); removeInviteErr != nil {
				resp.Diagnostics.AddError("failed to remove invitation", removeInviteErr.Error())
			}
		}
	}
}

// Read implements resource.Resource
func (r *UserGroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserGroupMembershipModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If the user doesn't exist, is no longer in the group, or there are no open invitations
	// then we should remove this resource from the state
	var usersResponse, getUserErr = client.GetUsersByEmail(ctx, r.qlient, data.Email.ValueString())

	if getUserErr != nil {
		resp.Diagnostics.AddError("failed to get user", getUserErr.Error())
		return
	}

	if len(usersResponse.IamGetUserList.Items) > 0 {
		var user = usersResponse.IamGetUserList.Items[0]

		// Iterate through groups and remove the user from the group
		for _, group := range user.UserGroups.Items {
			if group.Id == data.GroupId.ValueString() {
				// Membership exists, we can return early
				tflog.Trace(ctx, "User was found and is part of the group")
				data.Id = types.StringValue("placeholder")
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}
	} else {
		tflog.Trace(ctx, "User was not found, going to look through invitations")
	}

	// Lets see if the user is part of the group by open invitation
	var invitations, getInvitesErr = client.GetInvitations(ctx, r.qlient)

	if getInvitesErr != nil {
		resp.Diagnostics.AddError("failed to get invitations", getInvitesErr.Error())
		return
	}

	for _, invite := range invitations.IamGetAccount.AccountInvitations.Items {
		if invite.Email == data.Email.ValueString() && invite.GroupId == data.GroupId.ValueString() {
			// Membership exists, we can return early
			tflog.Trace(ctx, "Found invitation for user")
			data.Id = types.StringValue("placeholder")
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// If we reach this point, the user is not part of the group
	// and there are no open invitations, so we should remove this resource
	resp.State.RemoveResource(ctx)
}

// Update implements resource.Resource
func (r *UserGroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Get planned state
	var data UserGroupMembershipModel
	var currentState UserGroupMembershipModel

	// Read Terraform plan and state data into the models
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &currentState)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// We are going to remove the old user from the group and invite the new user

	// We need to delete the user from the group if the user is part of the group
	var usersResponse, getUserErr = client.GetUsersByEmail(ctx, r.qlient, currentState.Email.ValueString())

	if getUserErr != nil {
		resp.Diagnostics.AddError("failed to get user", getUserErr.Error())
		return
	}

	// User exists in this account, check for the group and remove them
	if len(usersResponse.IamGetUserList.Items) > 0 {
		var user = usersResponse.IamGetUserList.Items[0]

		// Iterate through groups and remove the user from the group
		for _, group := range user.UserGroups.Items {
			if group.Id == currentState.GroupId.ValueString() {
				if _, removeFromGroupErr := client.RemoveUserFromGroup(ctx, r.qlient, currentState.Email.ValueString(), group.Id); removeFromGroupErr != nil {
					resp.Diagnostics.AddError("failed to remove user from group", removeFromGroupErr.Error())
				}
				tflog.Trace(ctx, "User was removed from group",
					map[string]interface{}{"groupId": currentState.GroupId, "email": currentState.Email})
			}
		}
	}

	// Remove any invitations for the old user
	tflog.Trace(ctx, "User was not part of this group, removing any related invitations",
		map[string]interface{}{"groupId": currentState.GroupId, "email": currentState.Email})

	var invitations, getInvitesErr = client.GetInvitations(ctx, r.qlient)

	if getInvitesErr != nil {
		resp.Diagnostics.AddError("failed to get invitations", getInvitesErr.Error())
		return
	}

	for _, invite := range invitations.IamGetAccount.AccountInvitations.Items {
		if invite.Email == currentState.Email.ValueString() && invite.GroupId == currentState.GroupId.ValueString() {
			if _, removeInviteErr := client.RevokeInvitation(ctx, r.qlient, invite.Id); removeInviteErr != nil {
				resp.Diagnostics.AddError("failed to remove invitation", removeInviteErr.Error())
			}
			tflog.Trace(ctx, "Invitation was revoked",
				map[string]interface{}{"groupId": currentState.GroupId, "email": currentState.Email})
		}
	}

	// Now we can create the new user group membership
	_, err := client.InviteUser(
		ctx,
		r.qlient,
		data.Email.ValueString(),
		data.GroupId.ValueString(),
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to create user group membership", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	tflog.Trace(ctx, "Created user group membership",
		map[string]interface{}{"groupId": data.GroupId, "email": data.Email})
}
