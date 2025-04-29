package jupiterone

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &CustomIntegrationDefinitionResource{}

type CustomIntegrationDefinitionResource struct {
	version string
	qlient  graphql.Client
}

type CustomIntegrationDefinitionModel struct {
	Id                   types.String `json:"id,omitempty" tfsdk:"id"`
	Name                 types.String `json:"name" tfsdk:"name"`
	Description          types.String `json:"description" tfsdk:"description"`
	IntegrationType      types.String `json:"integrationType" tfsdk:"integration_type"`
	IntegrationCategory  types.List   `json:"integrationCategory" tfsdk:"integration_category"`
	Icon                 types.String `json:"icon" tfsdk:"icon"`
	DocsWebLink          types.String `json:"docsWebLink" tfsdk:"docs_web_link"`
	CustomDefinitionType types.String `json:"customDefinitionType" tfsdk:"custom_definition_type"`
}

func NewCustomIntegrationDefinitionResource() resource.Resource {
	return &CustomIntegrationDefinitionResource{}
}

func (*CustomIntegrationDefinitionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_integration_definition"
}

func (*CustomIntegrationDefinitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom integration definition in JupiterOne",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for the custom integration definition",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the custom integration definition",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Description of the custom integration definition",
			},
			"integration_type": schema.StringAttribute{
				Required:    true,
				Description: "Type of integration. Should be unique across JupiterOne. Should be a kebab-case string (lowercase with hyphens), e.g. 'jupiterone-example-integration'",
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`),
						"Integration type must be in kebab-case format (lowercase letters, numbers, and hyphens only)",
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"integration_category": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Category of integration",
			},
			"icon": schema.StringAttribute{
				Required:    true,
				Description: "Icon for the integration. Must be one of the preloaded icon names like 'custom_earth', 'custom_jupiter', etc. See custom integration definition UI for a full list of icons.",
			},
			"docs_web_link": schema.StringAttribute{
				Required:    true,
				Description: "Documentation web link for the integration",
			},
			"custom_definition_type": schema.StringAttribute{
				Required:    true,
				Description: "Type of custom definition. Must be either 'custom' or 'cft'",
				Validators: []validator.String{
					stringvalidator.OneOf(
						"custom",
						"cft",
					),
				},
			},
		},
	}
}

func (r *CustomIntegrationDefinitionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CustomIntegrationDefinitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CustomIntegrationDefinitionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var categories []string
	resp.Diagnostics.Append(data.IntegrationCategory.ElementsAs(ctx, &categories, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := client.CreateCustomIntegrationDefinitionInput{
		Name:                 data.Name.ValueString(),
		Description:          data.Description.ValueString(),
		IntegrationType:      data.IntegrationType.ValueString(),
		IntegrationCategory:  categories,
		Icon:                 data.Icon.ValueString(),
		DocsWebLink:          data.DocsWebLink.ValueString(),
		CustomDefinitionType: data.CustomDefinitionType.ValueString(),
	}

	created, err := client.CreateCustomIntegrationDefinition(ctx, r.qlient, input)
	if err != nil {
		resp.Diagnostics.AddError("failed to create custom integration definition", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateCustomIntegrationDefinition.Id)

	tflog.Trace(ctx, "Created custom integration definition",
		map[string]interface{}{"id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomIntegrationDefinitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CustomIntegrationDefinitionModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	def, err := client.GetCustomIntegrationDefinition(ctx, r.qlient, data.IntegrationType.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to get custom integration definition", err.Error())
		}
		return
	}

	data.Id = types.StringValue(def.CustomIntegrationDefinition.Id)
	data.Name = types.StringValue(def.CustomIntegrationDefinition.Name)
	data.Description = types.StringValue(def.CustomIntegrationDefinition.Description)
	data.IntegrationType = types.StringValue(def.CustomIntegrationDefinition.IntegrationType)

	var categoryValues []attr.Value
	for _, category := range def.CustomIntegrationDefinition.IntegrationCategory {
		categoryValues = append(categoryValues, types.StringValue(category))
	}
	data.IntegrationCategory = types.ListValueMust(types.StringType, categoryValues)

	data.Icon = types.StringValue(def.CustomIntegrationDefinition.Icon)
	data.DocsWebLink = types.StringValue(def.CustomIntegrationDefinition.DocsWebLink)
	// CustomDefinitionType is not available in the response type, so we'll skip setting it

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomIntegrationDefinitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CustomIntegrationDefinitionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var categories []string
	resp.Diagnostics.Append(data.IntegrationCategory.ElementsAs(ctx, &categories, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateInput := client.UpdateCustomIntegrationDefinitionInput{
		Name:                 data.Name.ValueString(),
		Description:          data.Description.ValueString(),
		IntegrationCategory:  categories,
		Icon:                 data.Icon.ValueString(),
		DocsWebLink:          data.DocsWebLink.ValueString(),
		CustomDefinitionType: data.CustomDefinitionType.ValueString(),
	}

	_, err := client.UpdateCustomIntegrationDefinition(ctx, r.qlient, data.Id.ValueString(), updateInput)
	if err != nil {
		resp.Diagnostics.AddError("failed to update custom integration definition", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomIntegrationDefinitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CustomIntegrationDefinitionModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.ArchiveCustomIntegrationDefinition(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to delete custom integration definition", err.Error())
	}
}
