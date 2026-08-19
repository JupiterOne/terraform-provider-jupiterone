package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type controlFrameworkRequirementStateModel struct {
	Id          types.String `tfsdk:"id"`
	Title       types.String `tfsdk:"title"`
	Description types.String `tfsdk:"description"`
	Identifier  types.String `tfsdk:"identifier"`
	Priority    types.String `tfsdk:"priority"`
	Section     types.String `tfsdk:"section"`
}

type controlFrameworkStateModel struct {
	Id              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	Source          types.String `tfsdk:"source"`
	SourceId        types.String `tfsdk:"source_id"`
	ResourceGroupId types.String `tfsdk:"resource_group_id"`
	Owner           types.String `tfsdk:"owner"`
	CreatedOn       types.String `tfsdk:"created_on"`
	Requirements    types.List   `tfsdk:"requirements"`
}

type controlFrameworksDataSourceModel struct {
	IncludeDeleted    types.Bool `tfsdk:"include_deleted"`
	ControlFrameworks types.List `tfsdk:"control_frameworks"`
}

func controlFrameworkRequirementObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the requirement. Use this to attach controls to the requirement.",
			},
			"title": schema.StringAttribute{
				Computed:    true,
				Description: "The title of the requirement.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the requirement.",
			},
			"identifier": schema.StringAttribute{
				Computed:    true,
				Description: "The identifier of the requirement.",
			},
			"priority": schema.StringAttribute{
				Computed:    true,
				Description: "The priority of the requirement: CRITICAL, HIGH, MEDIUM or LOW.",
			},
			"section": schema.StringAttribute{
				Computed:    true,
				Description: "The section of the framework this requirement belongs to.",
			},
		},
	}
}

func controlFrameworkObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the framework.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the framework.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the framework.",
			},
			"source": schema.StringAttribute{
				Computed:    true,
				Description: "Where the framework came from: J1, UCF, UPLOAD or JSON.",
			},
			"source_id": schema.StringAttribute{
				Computed:    true,
				Description: "The identifier of the framework in its source.",
			},
			"resource_group_id": schema.StringAttribute{
				Computed:    true,
				Description: "The resource group the framework is scoped to.",
			},
			"owner": schema.StringAttribute{
				Computed:    true,
				Description: "The owner of the framework, represented by a user email.",
			},
			"created_on": schema.StringAttribute{
				Computed:    true,
				Description: "When the framework was created.",
			},
			"requirements": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The requirements belonging to the framework.",
				NestedObject: controlFrameworkRequirementObject(),
			},
		},
	}
}

func NewControlFrameworksDataSource() datasource.DataSource {
	return &controlFrameworksDataSource{}
}

type controlFrameworksDataSource struct {
	version string
	qlient  graphql.Client
}

func (*controlFrameworksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_frameworks"
}

func (*controlFrameworksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The CCM frameworks in the account, together with their requirements. " +
			"This is how to reference a framework the provider does not manage, such as one imported " +
			"from the J1 or UCF catalog: read its requirement IDs here and attach managed controls to them.",
		Attributes: map[string]schema.Attribute{
			"include_deleted": schema.BoolAttribute{
				Optional:    true,
				Description: "Also return frameworks that have been deleted. Deletion in CCM is a soft delete.",
			},
			"control_frameworks": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The frameworks in the account.",
				NestedObject: controlFrameworkObject(),
			},
		},
	}
}

func (d *controlFrameworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data controlFrameworksDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	models := make([]controlFrameworkStateModel, 0)
	cursor := ""
	for {
		result, err := client.ListControlFrameworks(ctx, d.qlient, client.ControlFrameworksQueryInput{
			Cursor:         cursor,
			IncludeDeleted: data.IncludeDeleted.ValueBool(),
		})
		if err != nil {
			resp.Diagnostics.AddError("failed to list control frameworks", err.Error())
			return
		}

		for _, f := range result.ControlFrameworks.Items {
			requirements := make([]controlFrameworkRequirementStateModel, 0, len(f.Requirements))
			for _, r := range f.Requirements {
				requirements = append(requirements, controlFrameworkRequirementStateModel{
					Id:          types.StringValue(r.Id),
					Title:       stringOrNull(r.Title),
					Description: stringOrNull(r.Description),
					Identifier:  stringOrNull(r.Identifier),
					Priority:    stringOrNull(string(r.Priority)),
					Section:     stringOrNull(r.Section),
				})
			}

			requirementList, diags := types.ListValueFrom(ctx, controlFrameworkRequirementObject().Type(), requirements)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}

			models = append(models, controlFrameworkStateModel{
				Id:              types.StringValue(f.Id),
				Name:            types.StringValue(f.Name),
				Description:     stringOrNull(f.Description),
				Source:          stringOrNull(string(f.Source)),
				SourceId:        stringOrNull(f.SourceId),
				ResourceGroupId: stringOrNull(f.ResourceGroupId),
				Owner:           stringOrNull(f.Owner),
				CreatedOn:       stringOrNull(f.CreatedOn),
				Requirements:    requirementList,
			})
		}

		if !result.ControlFrameworks.PageInfo.HasNextPage || result.ControlFrameworks.PageInfo.EndCursor == "" {
			break
		}
		cursor = result.ControlFrameworks.PageInfo.EndCursor
	}

	frameworks, diags := types.ListValueFrom(ctx, controlFrameworkObject().Type(), models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ControlFrameworks = frameworks

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *controlFrameworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.version = p.version
	d.qlient = p.Qlient
}
