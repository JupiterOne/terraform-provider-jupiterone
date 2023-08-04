package jupiterone

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

var QueryResultsAre = []string{
	string(client.QueryResultsAreBad),
	string(client.QueryResultsAreGood),
	string(client.QueryResultsAreInformative),
	string(client.QueryResultsAreUnknown),
}

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &QuestionResource{}
var _ resource.ResourceWithConfigure = &QuestionResource{}
var _ resource.ResourceWithImportState = &QuestionResource{}

type QuestionResource struct {
	version string
	qlient  graphql.Client
}

type QuestionComplianceModel struct {
	Standard     string   `json:"standard" tfsdk:"standard"`
	Requirements []string `json:"requirements,omitempty" tfsdk:"requirements"`
	Controls     []string `json:"controls,omitempty" tfsdk:"controls"`
}

// QuestionQueryModel represents the terraform HCL `query` elements.
type QuestionQueryModel struct {
	// Query tests must be cleaned of carriage returns before being sent to
	// the server.
	Query           string `json:"query" tfsdk:"query"`
	Version         string `json:"version" tfsdk:"version"`
	Name            string `json:"name" tfsdk:"name"`
	IncludedDeleted bool   `json:"include_deleted" tfsdk:"include_deleted"`
	ResultsAre      string `json:"results_are" tfsdk:"results_are"`
}

// QuestionModel is the terraform HCL representation of a question. This
// currently has to be different from the `client.Question`:
//
//  1. allow the use of the `types.String` for ID being computed and the optional values
//  2. make it clearer where the line breaks are being stripped from the input
//     and state
//
// TODO: Unify the client types and the state model if possible
type QuestionModel struct {
	Id              types.String               `json:"id,omitempty" tfsdk:"id"`
	Title           types.String               `json:"title,omitempty" tfsdk:"title"`
	Description     types.String               `json:"description,omitempty" tfsdk:"description"`
	ShowTrend       types.Bool                 `json:"show_trend,omitempty" tfsdk:"show_trend"`
	PollingInterval types.String               `json:"polling_interval,omitempty" tfsdk:"polling_interval"`
	Tags            []string                   `json:"tags,omitempty" tfsdk:"tags"`
	Query           []*QuestionQueryModel      `json:"query,omitempty" tfsdk:"query"`
	Compliance      []*QuestionComplianceModel `json:"compliance,omitempty" tfsdk:"compliance"`
}

func NewQuestionResource() resource.Resource {
	return &QuestionResource{}
}

// Metadata implements resource.Resource
func (*QuestionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_question"
}

// Schema implements resource.Resource
func (*QuestionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne Question",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				Required:    true,
				Description: "The title of the question",
			},
			"description": schema.StringAttribute{
				Required: true,
			},
			"show_trend": schema.BoolAttribute{
				Description: "Whether to enable daily trend data collection. Defaults to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"polling_interval": schema.StringAttribute{
				Description: "Frequency of automated question evaluation. Defaults to ONE_DAY.",
				Computed:    true,
				Optional:    true,
				Default:     stringdefault.StaticString(string(client.SchedulerPollingIntervalOneDay)),
				Validators: []validator.String{
					stringvalidator.OneOf(PollingIntervals...),
				},
			},
			"tags": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
		// TODO: Deprecate the use of blocks following new framework guidance:
		// https://developer.hashicorp.com/terraform/plugin/framework/handling-data/blocks
		Blocks: map[string]schema.Block{
			"query": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(MIN_RULE_NAME_LENGTH, MAX_RULE_NAME_LENGTH),
							},
						},
						"query": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"version": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(MIN_RULE_NAME_LENGTH, MAX_RULE_NAME_LENGTH),
							},
						},
						"include_deleted": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(false),
						},
						"results_are": schema.StringAttribute{
							Description: "Defaults to INFORMATIVE.",
							Computed:    true,
							Optional:    true,
							Default:     stringdefault.StaticString(string(client.QueryResultsAreInformative)),
							Validators: []validator.String{
								stringvalidator.OneOf(QueryResultsAre...),
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"compliance": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"standard": schema.StringAttribute{
							Required: true,
						},
						"requirements": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
						"controls": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

// Configure implements resource.ResourceWithConfigure
func (r *QuestionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *QuestionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data QuestionModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	quest := data.BuildCreateQuestionInput()
	created, err := client.CreateQuestion(ctx, r.qlient, quest)

	if err != nil {
		resp.Diagnostics.AddError("failed to create question", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateQuestion.Id)

	tflog.Trace(ctx, "Created question",
		map[string]interface{}{"title": data.Title, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *QuestionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data QuestionModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteQuestion(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete question", err.Error())
	}
}

// Read implements resource.Resource
func (r *QuestionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data QuestionModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	q, err := client.GetQuestionById(ctx, r.qlient, data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get question", err.Error())
		return
	}

	data.Title = types.StringValue(q.Question.Title)
	data.Description = types.StringValue(q.Question.Description)
	data.Tags = q.Question.Tags
	data.PollingInterval = types.StringValue(string(q.Question.PollingInterval))

	// queries, which may have been stripped of line breaks
	data.Query = make([]*QuestionQueryModel, 0, len(q.Question.Queries))
	for _, query := range q.Question.Queries {
		data.Query = append(data.Query, &QuestionQueryModel{
			Query:           query.Query,
			Version:         query.Version,
			Name:            query.Name,
			ResultsAre:      string(query.ResultsAre),
			IncludedDeleted: query.IncludeDeleted,
		})
	}

	data.Compliance = make([]*QuestionComplianceModel, 0, len(q.Question.Compliance))
	for _, compliance := range q.Question.Compliance {
		data.Compliance = append(data.Compliance, &QuestionComplianceModel{
			Standard:     compliance.Standard,
			Requirements: compliance.Requirements,
			Controls:     compliance.Controls,
		})
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState
func (*QuestionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Update implements resource.Resource
func (r *QuestionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data QuestionModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	u := data.BuildQuestion()

	_, err := client.UpdateQuestion(ctx, r.qlient, data.Id.ValueString(), u)
	if err != nil {
		resp.Diagnostics.AddError("failed to update question", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated question",
		map[string]interface{}{"title": data.Title, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (qm *QuestionModel) BuildQuestion() client.QuestionUpdate {
	q := client.QuestionUpdate{
		Title:           qm.Title.ValueString(),
		Description:     qm.Description.ValueString(),
		Tags:            qm.Tags,
		ShowTrend:       qm.ShowTrend.ValueBool(),
		PollingInterval: client.SchedulerPollingInterval(qm.PollingInterval.ValueString()),
	}

	q.Queries = make([]client.QuestionQueryInput, 0, len(qm.Query))
	for _, query := range qm.Query {
		query.Query = removeCRFromString(query.Query)
		q.Queries = append(q.Queries, client.QuestionQueryInput{
			Query:          query.Query,
			Version:        query.Version,
			Name:           query.Name,
			ResultsAre:     client.QueryResultsAre(query.ResultsAre),
			IncludeDeleted: query.IncludedDeleted,
		})
	}

	q.Compliance = make([]client.QuestionComplianceMetaDataInput, 0, len(qm.Compliance))
	for _, compliance := range qm.Compliance {
		q.Compliance = append(q.Compliance, client.QuestionComplianceMetaDataInput{
			Standard:     compliance.Standard,
			Requirements: compliance.Requirements,
			Controls:     compliance.Controls,
		})
	}
	return q
}

func (qm *QuestionModel) BuildCreateQuestionInput() client.CreateQuestionInput {
	q := client.CreateQuestionInput{
		Title:           qm.Title.ValueString(),
		Description:     qm.Description.ValueString(),
		PollingInterval: client.SchedulerPollingInterval(qm.PollingInterval.ValueString()),
		ShowTrend:       qm.ShowTrend.ValueBool(),
		Tags:            qm.Tags,
	}

	q.Queries = make([]client.QuestionQueryInput, 0, len(qm.Query))
	for _, query := range qm.Query {
		query.Query = removeCRFromString(query.Query)
		q.Queries = append(q.Queries, client.QuestionQueryInput{
			Query:          query.Query,
			Version:        query.Version,
			Name:           query.Name,
			ResultsAre:     client.QueryResultsAre(query.ResultsAre),
			IncludeDeleted: query.IncludedDeleted,
		})
	}

	q.Compliance = make([]client.QuestionComplianceMetaDataInput, 0, len(qm.Compliance))
	for _, compliance := range qm.Compliance {
		q.Compliance = append(q.Compliance, client.QuestionComplianceMetaDataInput{
			Standard:     compliance.Standard,
			Requirements: compliance.Requirements,
			Controls:     compliance.Controls,
		})
	}
	return q
}
