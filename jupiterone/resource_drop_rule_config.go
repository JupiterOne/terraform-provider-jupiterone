package jupiterone

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

// The drop-rules config is a per-account singleton, so the resource uses a fixed
// import id.
const dropRuleConfigID = "drop-rule-config"

var _ resource.Resource = &DropRuleConfigResource{}
var _ resource.ResourceWithConfigure = &DropRuleConfigResource{}
var _ resource.ResourceWithImportState = &DropRuleConfigResource{}

type DropRuleConfigResource struct {
	version string
	qlient  graphql.Client
}

type dropRuleConditionModel struct {
	Property types.String `tfsdk:"property"`
	Op       types.String `tfsdk:"op"`
	Value    types.String `tfsdk:"value"`
}

type dropRuleModel struct {
	Id         types.String             `tfsdk:"id"`
	Enabled    types.Bool               `tfsdk:"enabled"`
	Type       types.String             `tfsdk:"type"`
	Class      types.String             `tfsdk:"class"`
	Conditions []dropRuleConditionModel `tfsdk:"conditions"`
}

type dropRuleConfigModel struct {
	Id      types.String    `tfsdk:"id"`
	Enabled types.Bool      `tfsdk:"enabled"`
	Version types.Int64     `tfsdk:"version"`
	Rules   []dropRuleModel `tfsdk:"rules"`
}

func NewDropRuleConfigResource() resource.Resource {
	return &DropRuleConfigResource{}
}

func (*DropRuleConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_drop_rule_config"
}

func (r *DropRuleConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (*DropRuleConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `Manages the account's drop-rule configuration (admin only).

Drop rules skip ingesting entities that match a rule during integration sync. There is
one drop-rule configuration per account (a singleton), so declare at most one
` + "`jupiterone_drop_rule_config`" + ` resource. Drop rules do NOT cover mapper/MRR entities; the
account owns the outcome of anything it drops.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Fixed identifier for the singleton config.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "Monotonic version of the configuration, bumped on every write.",
			},
			"enabled": schema.BoolAttribute{
				Required:    true,
				Description: "Master switch. When false, no rules are applied and nothing is dropped.",
			},
			"rules": schema.ListNestedAttribute{
				Optional:    true,
				Description: "The set of drop rules. An entity is dropped if it matches any enabled rule.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:    true,
							Description: "Stable identifier for the rule, unique within the config.",
						},
						"enabled": schema.BoolAttribute{
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(true),
							Description: "Whether this rule is applied. Defaults to true.",
						},
						"type": schema.StringAttribute{
							Optional:    true,
							Description: "Match only entities of this `_type`. Omit to match any type.",
						},
						"class": schema.StringAttribute{
							Optional:    true,
							Description: "Match only entities of this `_class`. Omit to match any class.",
						},
						"conditions": schema.ListNestedAttribute{
							Optional:    true,
							Description: "Property conditions, combined with logical AND.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"property": schema.StringAttribute{
										Required:    true,
										Description: "The integration-supplied property to test.",
									},
									"op": schema.StringAttribute{
										Required:    true,
										Description: "One of: eq, neq, in, exists, startsWith.",
										Validators: []validator.String{
											stringvalidator.OneOf("eq", "neq", "in", "exists", "startsWith"),
										},
									},
									"value": schema.StringAttribute{
										Optional: true,
										Description: "The comparison value, JSON-encoded (e.g. `jsonencode(false)`, " +
											"`jsonencode(\"prod\")`, `jsonencode([\"a\",\"b\"])`). Omit for `exists`.",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (*DropRuleConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildInput converts the terraform model into the GraphQL save input.
func buildDropRulesInput(data *dropRuleConfigModel) (client.DropRulesConfigInputBeta, error) {
	rules := make([]client.DropRuleInputBeta, 0, len(data.Rules))
	for _, rule := range data.Rules {
		conditions := make([]client.DropRuleConditionInputBeta, 0, len(rule.Conditions))
		for _, c := range rule.Conditions {
			value, err := decodeConditionValue(c.Value)
			if err != nil {
				return client.DropRulesConfigInputBeta{}, fmt.Errorf("rule %q condition %q: value must be valid JSON: %w", rule.Id.ValueString(), c.Property.ValueString(), err)
			}
			conditions = append(conditions, client.DropRuleConditionInputBeta{
				Property: c.Property.ValueString(),
				Op:       client.DropRuleOpBeta(c.Op.ValueString()),
				Value:    value,
			})
		}
		rules = append(rules, client.DropRuleInputBeta{
			Id:         rule.Id.ValueString(),
			Enabled:    rule.Enabled.ValueBool(),
			Type:       optionalString(rule.Type),
			Class:      optionalString(rule.Class),
			Conditions: conditions,
		})
	}
	return client.DropRulesConfigInputBeta{
		Enabled: data.Enabled.ValueBool(),
		Rules:   rules,
		// IfVersion intentionally omitted: Terraform owns this singleton, so
		// writes are last-writer-wins rather than compare-and-set.
	}, nil
}

// flatten copies the returned config into the terraform model.
func flattenDropRulesConfig(cfg client.DropRulesConfig, data *dropRuleConfigModel) {
	data.Id = types.StringValue(dropRuleConfigID)
	data.Enabled = types.BoolValue(cfg.Enabled)
	data.Version = types.Int64Value(cfg.Version)

	rules := make([]dropRuleModel, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		conditions := make([]dropRuleConditionModel, 0, len(rule.Conditions))
		for _, c := range rule.Conditions {
			conditions = append(conditions, dropRuleConditionModel{
				Property: types.StringValue(c.Property),
				Op:       types.StringValue(string(c.Op)),
				Value:    encodeConditionValue(c.Value),
			})
		}
		rules = append(rules, dropRuleModel{
			Id:         types.StringValue(rule.Id),
			Enabled:    types.BoolValue(rule.Enabled),
			Type:       stringOrNull(rule.Type),
			Class:      stringOrNull(rule.Class),
			Conditions: conditions,
		})
	}
	if len(rules) == 0 {
		rules = nil
	}
	data.Rules = rules
}

func (r *DropRuleConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data dropRuleConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.save(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	tflog.Trace(ctx, "Created drop rule config", map[string]interface{}{"version": data.Version.ValueInt64()})
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DropRuleConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data dropRuleConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.save(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// save issues the whole-config upsert and flattens the result back into data.
func (r *DropRuleConfigResource) save(ctx context.Context, data *dropRuleConfigModel, diags *diag.Diagnostics) {
	input, err := buildDropRulesInput(data)
	if err != nil {
		diags.AddError("Invalid drop rule config", err.Error())
		return
	}
	saved, err := client.SaveDropRulesConfig(ctx, r.qlient, input)
	if err != nil {
		diags.AddError("Failed to save drop rule config", err.Error())
		return
	}
	flattenDropRulesConfig(saved.SaveDropRulesConfigBeta.Config.DropRulesConfig, data)
}

func (r *DropRuleConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data dropRuleConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current, err := client.GetDropRulesConfig(ctx, r.qlient)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read drop rule config", err.Error())
		return
	}

	// A version of 0 means no config node exists (a real config is always >= 1).
	// The GraphQL field is nullable but genqlient returns a value struct, so we
	// key off the version to detect absence.
	if current.DropRulesConfigBeta.Version == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	flattenDropRulesConfig(current.DropRulesConfigBeta.DropRulesConfig, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete neutralizes the config (there is no delete-config GraphQL mutation):
// disable it and clear the rules so nothing is dropped.
func (r *DropRuleConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	_, err := client.SaveDropRulesConfig(ctx, r.qlient, client.DropRulesConfigInputBeta{
		Enabled: false,
		Rules:   []client.DropRuleInputBeta{},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete (disable) drop rule config", err.Error())
	}
}

// --- helpers ---

func optionalString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	if v == "" {
		return nil
	}
	return &v
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func decodeConditionValue(s types.String) (interface{}, error) {
	if s.IsNull() || s.IsUnknown() || s.ValueString() == "" {
		return nil, nil
	}
	var out interface{}
	if err := json.Unmarshal([]byte(s.ValueString()), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func encodeConditionValue(v interface{}) types.String {
	if v == nil {
		return types.StringNull()
	}
	b, err := json.Marshal(v)
	if err != nil {
		return types.StringNull()
	}
	return types.StringValue(string(b))
}
