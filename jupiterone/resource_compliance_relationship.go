package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

var ComplianceRelationshipTypes = []string{
	string(client.LibraryItemToFrameworkItemRelationshipTypeIgnoreEvidence),
	string(client.LibraryItemToFrameworkItemRelationshipTypeInheritEvidence),
}

type ComplianceRelationshipModel struct {
	Id               types.String `tfsdk:"id"`
	LibraryItemId    types.String `tfsdk:"library_item_id"`
	FrameworkItemId  types.String `tfsdk:"framework_item_id"`
	RelationshipType types.String `tfsdk:"relationship_type"`
}

type ComplianceRelationshipResource struct {
	version string
	qlient  graphql.Client
}

var _ resource.Resource = &ComplianceRelationshipResource{}
var _ resource.ResourceWithConfigure = &ComplianceRelationshipResource{}

func NewComplianceRelationshipResource() resource.Resource {
	return &ComplianceRelationshipResource{}
}

// Metadata implements resource.Resource.
func (*ComplianceRelationshipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_relationship"
}

func (r *ComplianceRelationshipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (*ComplianceRelationshipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A relationship between a framework item (requirement) and a library item (control).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"library_item_id": schema.StringAttribute{
				Required:    true,
				Description: "The id of the library item (control) to link",
				Validators: []validator.String{
					stringvalidator.LengthBetween(UUIDStrLength, UUIDStrLength),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"framework_item_id": schema.StringAttribute{
				Required:    true,
				Description: "The id of the framework item (requirement) to link",
				Validators: []validator.String{
					stringvalidator.LengthBetween(UUIDStrLength, UUIDStrLength),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"relationship_type": schema.StringAttribute{
				Required:    true,
				Description: "Whether to INHERIT_EVIDENCE or IGNORE_EVIDENCE in the linked framework item",
				Validators: []validator.String{
					stringvalidator.OneOf(ComplianceRelationshipTypes...),
				},
			},
		},
	}
}

// Create implements resource.Resource.
func (r *ComplianceRelationshipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ComplianceRelationshipModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.AttachComplianceLibraryItemToComplianceFrameworkItem(ctx, r.qlient, client.AttachComplianceLibraryItemToComplianceFrameworkItemInput{
		LibraryItemId:    data.LibraryItemId.ValueString(),
		FrameworkItemId:  data.FrameworkItemId.ValueString(),
		RelationshipType: client.LibraryItemToFrameworkItemRelationshipType(data.RelationshipType.ValueString()),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create compliance relationship", err.Error())
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%s-%s", data.LibraryItemId.ValueString(), data.FrameworkItemId.ValueString()))

	tflog.Trace(ctx, "Attached compliance relationship",
		map[string]interface{}{"library_item_id": data.LibraryItemId, "framework_item_id": data.FrameworkItemId})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource.
func (r *ComplianceRelationshipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ComplianceRelationshipModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.DetachComplianceLibraryItemFromComplianceFrameworkItem(ctx, r.qlient, client.DetachComplianceLibraryItemFromComplianceFrameworkItemInput{
		LibraryItemId:   data.LibraryItemId.ValueString(),
		FrameworkItemId: data.FrameworkItemId.ValueString(),
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to delete compliance relationship", err.Error())
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%s-%s", data.LibraryItemId.ValueString(), data.FrameworkItemId.ValueString()))

	tflog.Trace(ctx, "Detached compliance relationship",
		map[string]interface{}{"library_item_id": data.LibraryItemId, "framework_item_id": data.FrameworkItemId})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read implements resource.Resource.
func (r *ComplianceRelationshipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ComplianceRelationshipModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var i client.GetComplianceFrameworkItemRelationshipsByIdComplianceFrameworkItem
	if r, err := client.GetComplianceFrameworkItemRelationshipsById(ctx, r.qlient, data.FrameworkItemId.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to find framework item", err.Error())
		return
	} else {
		i = r.ComplianceFrameworkItem
	}

	data.RelationshipType = types.StringUnknown()
	for _, item := range i.LibraryItems.InheritedEvidenceLibraryItems {
		if item.Id == data.LibraryItemId.ValueString() {
			data.RelationshipType = types.StringValue(string(client.LibraryItemToFrameworkItemRelationshipTypeInheritEvidence))
			break
		}
	}
	for _, item := range i.LibraryItems.IgnoredEvidenceLibraryItems {
		if item.Id == data.LibraryItemId.ValueString() {
			data.RelationshipType = types.StringValue(string(client.LibraryItemToFrameworkItemRelationshipTypeIgnoreEvidence))
			break
		}
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update implements resource.Resource.
func (r *ComplianceRelationshipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ComplianceRelationshipModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.UpdateComplianceLibraryItemToComplianceFrameworkItemRelationship(ctx, r.qlient, client.UpdateComplianceLibraryItemToComplianceFrameworkItemRelationshipInput{
		FrameworkItemId: data.FrameworkItemId.ValueString(),
		LibraryItemId:   data.LibraryItemId.ValueString(),
		Updates: client.UpdateComplianceLibraryItemToComplianceFrameworkItemRelationshipFields{
			RelationshipType: client.LibraryItemToFrameworkItemRelationshipType(data.RelationshipType.ValueString()),
		},
	})

	if err != nil {
		resp.Diagnostics.AddError("failed to create framework", err.Error())
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%s-%s", data.LibraryItemId.ValueString(), data.FrameworkItemId.ValueString()))

	tflog.Trace(ctx, "Updated compliance relationship",
		map[string]interface{}{"library_item_id": data.LibraryItemId, "framework_item_id": data.FrameworkItemId})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
