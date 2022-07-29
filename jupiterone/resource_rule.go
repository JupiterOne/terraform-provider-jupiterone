package jupiterone

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
	"github.com/mitchellh/mapstructure"
)

const MIN_RULE_NAME_LENGTH = 1
const MAX_RULE_NAME_LENGTH = 255

func resourceQuestionRuleInstance() *schema.Resource {
	var RulePollingIntervals = []string{"DISABLED", "THIRTY_MINUTES", "ONE_HOUR", "ONE_DAY", "ONE_WEEK"}

	return &schema.Resource{
		CreateContext: resourceQuestionRuleInstanceCreate,
		ReadContext:   resourceQuestionRuleInstanceRead,
		UpdateContext: resourceQuestionRuleInstanceUpdate,
		DeleteContext: resourceQuestionRuleInstanceDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "Name of the rule, which is unique to each account.",
				ValidateFunc: validation.StringLenBetween(MIN_RULE_NAME_LENGTH, MAX_RULE_NAME_LENGTH),
			},
			"description": {
				Type:        schema.TypeString,
				Description: "Description of the rule",
				Required:    true,
			},
			"spec_version": {
				Type:        schema.TypeInt,
				Description: "Rule evaluation specification version in the case of breaking changes.",
				Default:     1,
				Optional:    true,
			},
			"version": {
				Type:        schema.TypeInt,
				Description: "Computed current version of the rule. Incremented each time the rule is updated.",
				Computed:    true,
			},
			"polling_interval": {
				Type:         schema.TypeString,
				Description:  "Frequency of automated rule evaluation. Defaults to ONE_DAY.",
				Default:      RulePollingIntervals[3],
				ValidateFunc: validation.StringInSlice(RulePollingIntervals, false),
				Optional:     true,
			},
			"templates": {
				Type:        schema.TypeMap,
				Description: "Optional key/value pairs of template name to template",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"question": {
				Type:        schema.TypeList,
				Description: "Contains properties related to queries used in the rule evaluation.",
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: getRuleQuestionSchema(),
				},
				AtLeastOneOf:  []string{"question", "question_id", "question_name"},
				ConflictsWith: []string{"question_id", "question_name"},
				Optional:      true,
			},
			"question_id": {
				Type:        schema.TypeString,
				Description: "Specifies the ID of a question to be used in rule evaluation.",
				Optional:    true,
			},
			"question_name": {
				Type:        schema.TypeString,
				Description: "Specifies the name of a question to be used in rule evaluation.",
				Optional:    true,
			},
			"operations": {
				Type:         schema.TypeString,
				Description:  "Actions that are executed when a corresponding condition is met.",
				ValidateFunc: validation.StringIsJSON,
				Required:     true,
			},
			"outputs": {
				Type:        schema.TypeList,
				Description: "Names of properties that can be used throughout the rule evaluation process and will be included in each record of a rule evaluation. (e.g. queries.query0.total)",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"tags": {
				Type:        schema.TypeList,
				Description: "Tags to apply to the rule.",
				Optional:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func getRuleQuestionSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"queries": {
			Type:     schema.TypeList,
			Required: true,
			Elem: &schema.Resource{
				Schema: getQuestionQuerySchema(),
			},
		},
	}
}

func getQuestionQuerySchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:     schema.TypeString,
			Optional: true,
		},
		"query": {
			Type:     schema.TypeString,
			Required: true,
		},
		"version": {
			Type:     schema.TypeString,
			Required: true,
		},
	}
}

func buildQuestionRuleInstanceProperties(d *schema.ResourceData) (*client.CommonQuestionRuleInstanceProperties, error) {
	var questionRuleInstance client.CommonQuestionRuleInstanceProperties

	if v, ok := d.GetOk("name"); ok {
		questionRuleInstance.Name = v.(string)
	}

	if v, ok := d.GetOk("description"); ok {
		questionRuleInstance.Description = v.(string)
	}

	if v, ok := d.GetOk("spec_version"); ok {
		questionRuleInstance.SpecVersion = v.(int)
	}

	if v, ok := d.GetOk("polling_interval"); ok {
		questionRuleInstance.PollingInterval = v.(string)
	}

	if v, ok := d.GetOk("outputs"); ok {
		questionRuleInstance.Outputs = interfaceSliceToStringSlice(v.([]interface{}))
	}

	if v, ok := d.GetOk("operations"); ok {
		questionRuleInstance.Operations = v.(string)
	} else {
		questionRuleInstance.Operations = "[]"
	}

	if v, ok := d.GetOk("question"); ok {
		ruleQuestion, err := buildQuestionRuleInstanceQuestion(v.([]interface{}))
		if err != nil {
			return nil, err
		}

		questionRuleInstance.Question = &(*ruleQuestion)[0]
	}

	if v, ok := d.GetOk("question_id"); ok {
		value := v.(string)
		questionRuleInstance.QuestionId = &value
	}

	if v, ok := d.GetOk("question_name"); ok {
		value := v.(string)
		questionRuleInstance.QuestionName = &value
	}

	if v, ok := d.GetOk("templates"); ok {
		questionRuleInstance.Templates = v.(map[string]interface{})
	}

	if v, ok := d.GetOk("tags"); ok {
		questionRuleInstance.Tags = interfaceSliceToStringSlice(v.([]interface{}))
	}

	return &questionRuleInstance, nil
}

func buildQuestionRuleInstanceQuestion(terraformRuleQuestionList []interface{}) (*[]client.RuleQuestion, error) {
	ruleQuestionList := make([]client.RuleQuestion, len(terraformRuleQuestionList))

	for i, terraformRuleQuestion := range terraformRuleQuestionList {
		var ruleQuestion client.RuleQuestion

		if err := mapstructure.Decode(terraformRuleQuestion, &ruleQuestion); err != nil {
			return nil, err
		}

		for i, query := range ruleQuestion.Queries {
			ruleQuestion.Queries[i].Query = removeCRFromString(query.Query)
		}

		ruleQuestionList[i] = ruleQuestion
	}

	return &ruleQuestionList, nil
}

func resourceQuestionRuleInstanceCreate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	questionRuleInstanceProperties, err := buildQuestionRuleInstanceProperties(d)
	if err != nil {
		return diag.Errorf("failed to build question rule instance: %s", err.Error())
	}

	createdQuestion, err := m.(*ProviderConfiguration).Client.CreateQuestionRuleInstance(*questionRuleInstanceProperties)
	if err != nil {
		return diag.Errorf("failed to create question rule instance: %s", err.Error())
	}

	if err := d.Set("version", createdQuestion.Version); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(createdQuestion.Id)
	return nil
}

func resourceQuestionRuleInstanceUpdate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	questionRuleInstanceProperties, err := buildQuestionRuleInstanceProperties(d)
	if err != nil {
		return diag.Errorf("failed to build question rule instance: %s", err.Error())
	}

	var updateQuestionRuleInstanceProperties client.UpdateQuestionRuleInstanceProperties
	updateQuestionRuleInstanceProperties.Id = d.Id()
	updateQuestionRuleInstanceProperties.Name = questionRuleInstanceProperties.Name
	updateQuestionRuleInstanceProperties.Description = questionRuleInstanceProperties.Description
	updateQuestionRuleInstanceProperties.SpecVersion = questionRuleInstanceProperties.SpecVersion
	updateQuestionRuleInstanceProperties.PollingInterval = questionRuleInstanceProperties.PollingInterval
	updateQuestionRuleInstanceProperties.Operations = questionRuleInstanceProperties.Operations
	updateQuestionRuleInstanceProperties.Outputs = questionRuleInstanceProperties.Outputs
	updateQuestionRuleInstanceProperties.Question = questionRuleInstanceProperties.Question
	updateQuestionRuleInstanceProperties.Templates = questionRuleInstanceProperties.Templates
	updateQuestionRuleInstanceProperties.Tags = questionRuleInstanceProperties.Tags

	if v, ok := d.GetOk("version"); ok {
		updateQuestionRuleInstanceProperties.Version = v.(int)
	}

	updatedQuestionRuleInstance, err := m.(*ProviderConfiguration).Client.UpdateQuestionRuleInstance(updateQuestionRuleInstanceProperties)

	if err != nil {
		return diag.Errorf("failed to update question rule instance: %s", err.Error())
	}

	if err := d.Set("version", updatedQuestionRuleInstance.Version); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceQuestionRuleInstanceDelete(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := m.(*ProviderConfiguration).Client.DeleteQuestionRuleInstance(d.Id()); err != nil {
		return diag.Errorf("failed to delete question rule instance: %s", err.Error())
	}

	d.SetId("")
	return nil
}

func resourceQuestionRuleInstanceRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	questionRuleInstance, err := m.(*ProviderConfiguration).Client.GetQuestionRuleInstanceByID(d.Id())

	if err != nil {
		return diag.Errorf("failed to read existing question rule instance: %s", err.Error())
	}

	if err := d.Set("version", questionRuleInstance.Version); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(questionRuleInstance.Id)
	return nil
}
