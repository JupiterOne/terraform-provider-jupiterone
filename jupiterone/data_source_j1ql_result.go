package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type QueryModel struct {
	Query          types.String `json:"query,omitempty" tfsdk:"query"`
	IncludeDeleted types.Bool   `json:"includeDeleted,omitempty" tfsdk:"include_deleted"`
}

type J1QLResultModel struct {
	Id       types.String `json:"id,omitempty" tfsdk:"id"`
	Query    QueryModel   `json:"query,omitempty" tfsdk:"query"`
	Type     types.String `json:"type,omitempty" tfsdk:"type"`
	DataJson types.String `json:"data,omitempty" tfsdk:"data_json"`
}

// NewJ1QLDataSource is a helper function to simplify the provider implementation.
func NewJ1QLResultDataSource() datasource.DataSource {
	return &j1qlResultDataSource{}
}

// j1qlResultDataSource is the data source implementation.
type j1qlResultDataSource struct {
	version string
	qlient  graphql.Client
}

// Metadata implements resource.Resource
func (*j1qlResultDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_j1ql_result"
}

// Schema implements resource.Resource
func (*j1qlResultDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A j1ql query result.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"query": schema.SingleNestedAttribute{
				Required:    true,
				Description: "The query object to execute.",
				Attributes: map[string]schema.Attribute{
					"query": schema.StringAttribute{
						Required:    true,
						Description: "The j1ql query string.",
					},
					"include_deleted": schema.BoolAttribute{
						Optional:    true,
						Description: "Whether to include deleted entities in the results.",
					},
				},
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "The return type of the query. Possible values are: list, table and tree.",
			},
			"data_json": schema.StringAttribute{
				Description: "The json stringified data that was returned for the query.",
				Computed:    true,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *j1qlResultDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data J1QLResultModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	executeResponse, err := client.ExecuteQuery(ctx, d.qlient, data.Query.Query.ValueString(), data.Query.IncludeDeleted.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("failed to execute queryquery", err.Error())
		return
	}

	fmt.Sprintf("The length of the data %d", len(getFormattedData(executeResponse.QueryV1.Data)))

	stringifiedData, err := json.Marshal(executeResponse.QueryV1.Data)
	fmt.Printf("Stringified data id" + string(stringifiedData))

	if err != nil {
		resp.Diagnostics.AddError("failed to marshal query data", err.Error())
		return
	}

	data.Id = types.StringValue(uuid.New().String())
	data.Type = types.StringValue(executeResponse.QueryV1.Type)
	data.DataJson = types.StringValue(string(stringifiedData))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Configure implements resource.ResourceWithConfigure
func (r *j1qlResultDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func getFormattedData(data interface{}) []map[string]string {
	if dataArray, ok := data.([]interface{}); ok {
		result := make([]map[string]string, len(dataArray))

		for i, item := range dataArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				result[i] = make(map[string]string)
				for key, value := range itemMap {
					// result[i][key] = fmt.Sprintf("%v", value)
					switch v := value.(type) {
					case bool:
						result[i][key] = strconv.FormatBool(v)
					case float64:
						result[i][key] = strconv.FormatFloat(v, 'f', -1, 64)
					case int16, int32, int64:
						result[i][key] = fmt.Sprintf("%d", v)
					case string:
						result[i][key] = v
					default:
						result[i][key] = "Redacted: Not a string or boolean or number"
					}
				}
			}
		}
		return result
	}

	return []map[string]string{}
}
