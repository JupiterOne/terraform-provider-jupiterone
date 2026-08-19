package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type controlFrameworkSectionStatsStateModel struct {
	SectionName                 types.String `tfsdk:"section_name"`
	NumberOfRequirements        types.Int64  `tfsdk:"number_of_requirements"`
	NumberOfPassingRequirements types.Int64  `tfsdk:"number_of_passing_requirements"`
	NumberOfFailingRequirements types.Int64  `tfsdk:"number_of_failing_requirements"`
}

type controlFrameworkStatsStateModel struct {
	Id                                  types.String `tfsdk:"id"`
	NumberOfControls                    types.Int64  `tfsdk:"number_of_controls"`
	NumberOfFailingControls             types.Int64  `tfsdk:"number_of_failing_controls"`
	NumberOfPassingControls             types.Int64  `tfsdk:"number_of_passing_controls"`
	NumberOfAttestedControls            types.Int64  `tfsdk:"number_of_attested_controls"`
	NumberOfRequirements                types.Int64  `tfsdk:"number_of_requirements"`
	NumberOfControlsWithTests           types.Int64  `tfsdk:"number_of_controls_with_tests"`
	NumberOfPassingRequirements         types.Int64  `tfsdk:"number_of_passing_requirements"`
	NumberOfFailingRequirements         types.Int64  `tfsdk:"number_of_failing_requirements"`
	NumberOfFailingCriticalRequirements types.Int64  `tfsdk:"number_of_failing_critical_requirements"`
	SectionStats                        types.List   `tfsdk:"section_stats"`
}

type controlFrameworkStatsDataSourceModel struct {
	Id           types.String `tfsdk:"id"`
	FrameworkIds types.List   `tfsdk:"framework_ids"`
	Stats        types.List   `tfsdk:"stats"`
}

func controlFrameworkSectionStatsObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"section_name": schema.StringAttribute{
				Computed:    true,
				Description: "The name of the section.",
			},
			"number_of_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many requirements the section contains.",
			},
			"number_of_passing_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the section's requirements are passing.",
			},
			"number_of_failing_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the section's requirements are failing.",
			},
		},
	}
}

func controlFrameworkStatsObject() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The ID of the framework these statistics describe.",
			},
			"number_of_controls": schema.Int64Attribute{
				Computed:    true,
				Description: "How many controls the framework contains.",
			},
			"number_of_failing_controls": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the framework's controls are failing.",
			},
			"number_of_passing_controls": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the framework's controls are passing.",
			},
			"number_of_attested_controls": schema.Int64Attribute{
				Computed: true,
				Description: "How many controls are covered by a valid attestation. A subset of " +
					"number_of_passing_controls, reported separately so attestation coverage is visible " +
					"distinctly from raw pass and fail counts.",
			},
			"number_of_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many requirements the framework contains.",
			},
			"number_of_controls_with_tests": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the framework's controls have at least one control test.",
			},
			"number_of_passing_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the framework's requirements are passing.",
			},
			"number_of_failing_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the framework's requirements are failing.",
			},
			"number_of_failing_critical_requirements": schema.Int64Attribute{
				Computed:    true,
				Description: "How many of the framework's CRITICAL priority requirements are failing.",
			},
			"section_stats": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "Per-section requirement statistics.",
				NestedObject: controlFrameworkSectionStatsObject(),
			},
		},
	}
}

func NewControlFrameworkStatsDataSource() datasource.DataSource {
	return &controlFrameworkStatsDataSource{}
}

type controlFrameworkStatsDataSource struct {
	version string
	qlient  graphql.Client
}

func (*controlFrameworkStatsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_control_framework_stats"
}

func (*controlFrameworkStatsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Compliance statistics for one or more CCM frameworks: the scorecard numbers behind " +
			"the framework compliance view." +
			"\n\nThese results are read from the graph, which is eventually consistent. An object " +
			"created earlier in the same apply may not appear yet; a later plan or apply will show " +
			"it. Do not rely on this data source to observe a change made moments earlier in the " +
			"same run.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "A generated identifier for this result set.",
			},
			"framework_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "The frameworks to report on. With none given, every framework is reported.",
			},
			"stats": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "Statistics per framework.",
				NestedObject: controlFrameworkStatsObject(),
			},
		},
	}
}

func (d *controlFrameworkStatsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data controlFrameworkStatsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	frameworkIds, diags := stringSlice(ctx, data.FrameworkIds)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := client.GetControlFrameworkStats(ctx, d.qlient, client.ControlFrameworkStatsInput{
		FrameworkIds: frameworkIds,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to get control framework stats", err.Error())
		return
	}

	models := make([]controlFrameworkStatsStateModel, 0, len(result.ControlFrameworkStats))
	for _, s := range result.ControlFrameworkStats {
		sections := make([]controlFrameworkSectionStatsStateModel, 0, len(s.SectionStats))
		for _, sec := range s.SectionStats {
			sections = append(sections, controlFrameworkSectionStatsStateModel{
				SectionName:                 types.StringValue(sec.SectionName),
				NumberOfRequirements:        types.Int64Value(int64(sec.NumberOfRequirements)),
				NumberOfPassingRequirements: types.Int64Value(int64(sec.NumberOfPassingRequirements)),
				NumberOfFailingRequirements: types.Int64Value(int64(sec.NumberOfFailingRequirements)),
			})
		}

		sectionList, diags := types.ListValueFrom(ctx, controlFrameworkSectionStatsObject().Type(), sections)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		models = append(models, controlFrameworkStatsStateModel{
			Id:                                  types.StringValue(s.Id),
			NumberOfControls:                    types.Int64Value(int64(s.NumberOfControls)),
			NumberOfFailingControls:             types.Int64Value(int64(s.NumberOfFailingControls)),
			NumberOfPassingControls:             types.Int64Value(int64(s.NumberOfPassingControls)),
			NumberOfAttestedControls:            types.Int64Value(int64(s.NumberOfAttestedControls)),
			NumberOfRequirements:                types.Int64Value(int64(s.NumberOfRequirements)),
			NumberOfControlsWithTests:           types.Int64Value(int64(s.NumberOfControlsWithTests)),
			NumberOfPassingRequirements:         types.Int64Value(int64(s.NumberOfPassingRequirements)),
			NumberOfFailingRequirements:         types.Int64Value(int64(s.NumberOfFailingRequirements)),
			NumberOfFailingCriticalRequirements: types.Int64Value(int64(s.NumberOfFailingCriticalRequirements)),
			SectionStats:                        sectionList,
		})
	}

	stats, diags := types.ListValueFrom(ctx, controlFrameworkStatsObject().Type(), models)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Stats = stats
	data.Id = types.StringValue(uuid.New().String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *controlFrameworkStatsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
