package jupiterone

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

type AccountParameterResource struct {
	version string
	qlient  graphql.Client
}

// AccountParameterModel is the terraform HCL representation of an account parameter.
type AccountParameterModel struct {
	Id        types.String `json:"id,omitempty" tfsdk:"id"`
	Name      types.String `json:"name,omitempty" tfsdk:"name"`
	Value     types.String `json:"value,omitempty" tfsdk:"value"`
	ValueType types.String `json:"valueType,omitempty" tfsdk:"value_type"`
	Secret    types.Bool   `json:"secret,omitempty" tfsdk:"secret"`
}

func NewAccountParameterResource() resource.Resource {
	return &AccountParameterResource{}
}

// Metadata implements resource.Resource
func (*AccountParameterResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_parameter"
}

// Schema implements resource.Resource
func (*AccountParameterResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A saved JupiterOne Account Parameter.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the account parameter. Must be unique. Must contain no spaces, just alphanumeric characters, and underscores.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"value": schema.StringAttribute{
				Required:    true,
				Description: "The value of the account parameter. This string value gets parsed based on the value_type.",
			},
			"value_type": schema.StringAttribute{
				Required:    true,
				Description: "The type of the value.",
				Validators: []validator.String{
					stringvalidator.OneOf("string", "number", "boolean"),
				},
			},
			"secret": schema.BoolAttribute{
				Description: "Wether or not the value is secret. Defaults to false. If it is secret then it cannot be retrieved through the API and will show as changed for every terraform plan.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

// Configure implements resource.ResourceWithConfigure
func (r *AccountParameterResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *AccountParameterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccountParameterModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var parsedValue, parseError = parseValue(data.ValueType.ValueString(), data.Value.ValueString())

	if parseError != nil {
		resp.Diagnostics.AddError("failed to parse account parameter value", parseError.Error())
		return
	}

	created, err := client.SetAccountParameter(
		ctx,
		r.qlient,
		data.Name.ValueString(),
		parsedValue,
		data.Secret.ValueBool(),
	)

	if err != nil || !created.SetParameter.Success {
		resp.Diagnostics.AddError("failed to create account parameter", err.Error())
		return
	}

	data.Id = types.StringValue(data.Name.ValueString())

	tflog.Trace(ctx, "Created account parameter",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete implements resource.Resource
func (r *AccountParameterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccountParameterModel

	// Read Terraform ste into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := client.DeleteAccountParameter(ctx, r.qlient, data.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("failed to delete account parameter", err.Error())
	}
}

// Read implements resource.Resource
func (r *AccountParameterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccountParameterModel

	// Read Terraform state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	parameterResp, err := client.GetAccountParameter(ctx, r.qlient, data.Name.ValueString())
	log.Println("Read account parameter:", parameterResp.GetParameter().Name == "")

	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("failed to get account parameter", err.Error())
		}
		return
	} else if parameterResp.Parameter.Name == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Name = types.StringValue(parameterResp.Parameter.Name)
	data.Value = types.StringValue(parseValueAsString(parameterResp.Parameter.Value))
	data.ValueType = types.StringValue(determineValueType(parameterResp.Parameter.Value))
	data.Secret = types.BoolValue(parameterResp.Parameter.Secret)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// ImportState implements resource.ResourceWithImportState
func (*AccountParameterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Update implements resource.Resource
func (r *AccountParameterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AccountParameterModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var parsedValue, parseError = parseValue(data.ValueType.ValueString(), data.Value.ValueString())

	if parseError != nil {
		resp.Diagnostics.AddError("failed to parse account parameter value", parseError.Error())
		return
	}

	_, err := client.SetAccountParameter(
		ctx,
		r.qlient,
		data.Name.ValueString(),
		parsedValue,
		data.Secret.ValueBool(),
	)

	if err != nil {
		resp.Diagnostics.AddError("failed to update account parameter", err.Error())
		return
	}

	tflog.Trace(ctx, "Updated account parameter",
		map[string]interface{}{"title": data.Name, "id": data.Id})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Parse the value based on the value type like in the update and create functions above
func parseValue(valueType string, value string) (interface{}, error) {
	if valueType == "boolean" {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return boolValue, nil
	}

	if valueType == "number" {
		numValue, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return numValue, nil
	}

	return value, nil
}

// Parse a dynamic interface{} value back to a string
// value type should not be a param for the function, the function should determine what type of a value interface{} is
func parseValueAsString(value interface{}) string {
	switch v := value.(type) {
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case string:
		return v
	default:
		return ""
	}
}

// Now I want to return "boolean", "number", or "string" based on the given value interface{}
func determineValueType(value interface{}) string {
	switch value.(type) {
	case bool:
		return "boolean"
	case float64, int, int8, int16, int32, int64:
		return "number"
	case string:
		return "string"
	default:
		return "string" // default to string if type is unknown
	}
}
