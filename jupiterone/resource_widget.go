package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"

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

var WidgetTypes = []string{
	string("bar"),
	string("number"),
	string("pie"),
	string("status"),
	string("markdown"),
	string("graph"),
	string("line"),
	string("matrix"),
	string("area"),
	string("table"),
}

type WidgetResource struct {
	version string
	qlient  graphql.Client
}

type WidgetQuery struct {
	Name  types.String `json:"name" tfsdk:"name"`
	Query types.String `json:"query" tfsdk:"query"`
}

type WidgetConfig struct {
	Queries  []WidgetQuery `json:"queries,omitempty" tfsdk:"queries"`
	Settings types.String  `json:"settings,omitempty" tfsdk:"settings"`
}

type WidgetModel struct {
	Id          types.String `json:"id,omitempty" tfsdk:"id"`
	Title       types.String `json:"title,omitempty" tfsdk:"title"`
	Description types.String `json:"description,omitempty" tfsdk:"description"`
	Type        types.String `json:"type" tfsdk:"type"`
	DashboardId types.String `json:"dashboard_id" tfsdk:"dashboard_id"`
	Config      WidgetConfig `json:"config" tfsdk:"config"`
}

func NewWidgetResource() resource.Resource {
	return &WidgetResource{}
}

func (*WidgetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_widget"
}

func (r *WidgetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *WidgetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *WidgetModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	widgetInput, err := data.BuildCreateInsightsWidgetInput()
	if err != nil {
		resp.Diagnostics.AddError("failed to build widget input from configuration", err.Error())
		return
	}

	created, err := client.CreateWidget(
		ctx,
		r.qlient,
		data.DashboardId.ValueString(),
		widgetInput,
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to create widget entity", err.Error())
		return
	}

	data.Id = types.StringValue(created.CreateWidget.Id)

	tflog.Trace(ctx, "Created widget",
		map[string]interface{}{"title": data.Title, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

// Delete implements resource.Resource.
func (r *WidgetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *WidgetModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteWidget(ctx, r.qlient, data.DashboardId.ValueString(), data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete widget", err.Error())
	}
}

// Read implements resource.Resource.
func (r *WidgetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WidgetModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch the widget data from the API
	response, err := client.GetWidget(ctx, r.qlient, data.DashboardId.ValueString(), "Account", data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to get widget", err.Error())
		return
	}

	// Marshal API response to JSON
	jsonData, err := json.Marshal(response.GetWidget.Widget)
	if err != nil {
		resp.Diagnostics.AddError("failed to marshal json from widget api", err.Error())
		return
	}

	// Unmarshal JSON response into a map
	var widgetMap map[string]interface{}
	err = json.Unmarshal(jsonData, &widgetMap)
	if err != nil {
		resp.Diagnostics.AddError("failed to unmarshal widget settings", err.Error())
		return
	}

	// Process the config field
	var widgetConfig WidgetConfig
	if config, ok := widgetMap["config"].(map[string]interface{}); ok {
		// Convert settings to JSON string
		if settings, ok := config["settings"]; ok {
			settingsJson, err := json.Marshal(settings)
			if err != nil {
				resp.Diagnostics.AddError("failed to marshal settings to JSON", err.Error())
				return
			}
			if string(settingsJson) != "{}" {
				widgetConfig.Settings = types.StringValue(string(settingsJson))
			} else {
				widgetConfig.Settings = types.StringNull()
			}
		}

		// Process queries
		if queries, ok := config["queries"].([]interface{}); ok {
			widgetConfig.Queries = make([]WidgetQuery, len(queries))
			for i, query := range queries {
				if queryMap, ok := query.(map[string]interface{}); ok {
					var widgetQuery WidgetQuery
					if name, ok := queryMap["name"].(string); ok {
						widgetQuery.Name = types.StringValue(name)
					}
					if queryString, ok := queryMap["query"].(string); ok {
						widgetQuery.Query = types.StringValue(queryString)
					}
					widgetConfig.Queries[i] = widgetQuery
				}
			}
		}
	}

	// Map the widgetMap to WidgetModel
	data.Id = types.StringValue(widgetMap["id"].(string))
	data.Title = types.StringValue(widgetMap["title"].(string))
	data.Description = types.StringValue(widgetMap["description"].(string))
	data.Type = types.StringValue(widgetMap["type"].(string))
	data.Config = widgetConfig

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState
func (*WidgetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Schema implements resource.Resource.
func (*WidgetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A JupiterOne insights widget.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"title": schema.StringAttribute{
				Required:    true,
				Description: "The title of the widget.",
			},
			"dashboard_id": schema.StringAttribute{
				Required:    true,
				Description: "The ID for the dashboard where the widget will be added.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the widget.",
				Validators: []validator.String{
					stringvalidator.OneOf(WidgetTypes...),
				},
			},
			"config": schema.SingleNestedAttribute{
				Required:    true,
				Description: "The configuration properties for the widget.",
				Attributes: map[string]schema.Attribute{
					"settings": schema.StringAttribute{
						Optional:    true,
						Description: "The settings for the widget. This is a flexible JSON structure.",
					},
					"queries": schema.ListNestedAttribute{
						Description: "Queries used to power the widget.",
						Required:    true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Optional:    true,
									Description: "The query name.",
								},
								"query": schema.StringAttribute{
									Required:    true,
									Description: "The query.",
								},
							},
						},
					},
				},
			},
			"description": schema.StringAttribute{
				Description: "The description for widget.",
				Optional:    true,
			},
		},
	}
}

// Update implements resource.Resource.
func (r *WidgetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *WidgetModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	settings := map[string]interface{}{}

	if data.Config.Settings.IsNull() {
		settings = make(map[string]interface{})
	} else {
		err := json.Unmarshal([]byte(data.Config.Settings.ValueString()), &settings)
		if err != nil {
			resp.Diagnostics.AddError("failed to unencode widget settings", err.Error())
		}
	}

	queries := make([]client.WidgetQuery, len(data.Config.Queries))
	for i, query := range data.Config.Queries {
		queries[i] = client.WidgetQuery{
			Name:  query.Name.ValueString(),
			Query: query.Query.ValueString(),
		}
	}

	config := client.WidgetConfig{
		Queries:  queries,
		Settings: settings,
	}
	widgetInput := client.Widget{
		Id:          data.Id.ValueString(),
		Title:       data.Title.ValueString(),
		Description: data.Description.ValueString(),
		Type:        data.Type.ValueString(),
		Config:      config,
	}

	_, err := client.UpdateWidget(
		ctx,
		r.qlient,
		data.DashboardId.ValueString(),
		"Account",
		widgetInput,
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to update widget", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated widget",
		map[string]interface{}{"title": data.Title, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WidgetModel) BuildCreateInsightsWidgetInput() (client.CreateInsightsWidgetInput, error) {

	settings := map[string]interface{}{}

	if r.Config.Settings.IsNull() {
		settings = make(map[string]interface{})
	} else {
		err := json.Unmarshal([]byte(r.Config.Settings.ValueString()), &settings)
		if err != nil {
			return client.CreateInsightsWidgetInput{}, err
		}
	}

	queries := make([]client.CreateInsightsWidgetConfigQueryInput, len(r.Config.Queries))
	for i, query := range r.Config.Queries {
		queries[i] = client.CreateInsightsWidgetConfigQueryInput{
			Name:  query.Name.ValueString(),
			Query: query.Query.ValueString(),
		}
	}

	config := client.CreateInsightsWidgetConfigInput{
		Queries:  queries,
		Settings: settings,
	}

	widget := client.CreateInsightsWidgetInput{
		Id:          r.Id.ValueString(),
		Title:       r.Title.ValueString(),
		Description: r.Description.ValueString(),
		Type:        r.Type.ValueString(),
		Config:      config,
	}

	return widget, nil
}
