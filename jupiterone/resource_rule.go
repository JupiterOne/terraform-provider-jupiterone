package jupiterone

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/mitchellh/mapstructure"

	client "github.com/jupiterone/terraform-provider-jupiterone/jupiterone_client"
	jupiterone "github.com/jupiterone/terraform-provider-jupiterone/jupiterone_client"
)

func resourceQuestionRuleInstance() *schema.Resource {
	var RulePollingIntervals = []string{"DISABLED", "THIRTY_MINUTES", "ONE_HOUR", "ONE_DAY"}

	return &schema.Resource{
		Create: resourceQuestionRuleInstanceCreate,
		Read:   resourceQuestionRuleInstanceRead,
		Update: resourceQuestionRuleInstanceUpdate,
		Delete: resourceQuestionRuleInstanceDelete,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "Name of the rule, which is unique to each account.",
				ValidateFunc: validation.StringLenBetween(1, 255),
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
				Required:    true,
				Elem: &schema.Resource{
					Schema: getRuleQuestionSchema(),
				},
			},
			"operations": {
				Type:         schema.TypeString,
				Description:  "Actions that are executed when a corresponding condition is met.",
				ValidateFunc: validation.ValidateJsonString,
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

func buildQuestionRuleInstanceProperties(d *schema.ResourceData) (*client.BaseQuestionRuleInstanceProperties, error) {
	var questionRuleInstance client.BaseQuestionRuleInstanceProperties

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
		questionRuleInstance.Outputs = buildQuestionRuleInstanceOutputs(v.([]interface{}))
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

		questionRuleInstance.Question = (*ruleQuestion)[0]
	}

	if v, ok := d.GetOk("templates"); ok {
		questionRuleInstance.Templates = v.(map[string]interface{})
	}

	return &questionRuleInstance, nil
}

func buildQuestionRuleInstanceOutputs(terraformOutputsList []interface{}) []string {
	outputList := make([]string, len(terraformOutputsList))

	for i, output := range terraformOutputsList {
		outputList[i] = output.(string)
	}

	return outputList
}

func buildQuestionRuleInstanceQuestion(terraformRuleQuestionList []interface{}) (*[]jupiterone.RuleQuestion, error) {
	ruleQuestionList := make([]jupiterone.RuleQuestion, len(terraformRuleQuestionList))

	for i, terraformRuleQuestion := range terraformRuleQuestionList {
		var ruleQuestion jupiterone.RuleQuestion

		if err := mapstructure.Decode(terraformRuleQuestion, &ruleQuestion); err != nil {
			return nil, err
		}

		ruleQuestionList[i] = ruleQuestion
	}

	return &ruleQuestionList, nil
}

func resourceQuestionRuleInstanceCreate(d *schema.ResourceData, m interface{}) error {
	questionRuleInstanceProperties, err := buildQuestionRuleInstanceProperties(d)
	if err != nil {
		return fmt.Errorf("Failed to build question rule instance: %s", err.Error())
	}

	createdQuestion, err := m.(*ProviderConfiguration).Client.CreateQuestionRuleInstance(*questionRuleInstanceProperties)
	if err != nil {
		return fmt.Errorf("Failed to create question rule instance: %s", err.Error())
	}

	d.Set("version", createdQuestion.Version)
	d.SetId(createdQuestion.Id)

	return nil
}

func resourceQuestionRuleInstanceUpdate(d *schema.ResourceData, m interface{}) error {
	questionRuleInstanceProperties, err := buildQuestionRuleInstanceProperties(d)
	if err != nil {
		return fmt.Errorf("Failed to build question rule instance: %s", err.Error())
	}

	var updateQuestionRuleInstanceProperties jupiterone.UpdateQuestionRuleInstanceProperties
	updateQuestionRuleInstanceProperties.Id = d.Id()
	updateQuestionRuleInstanceProperties.Name = questionRuleInstanceProperties.Name
	updateQuestionRuleInstanceProperties.Description = questionRuleInstanceProperties.Description
	updateQuestionRuleInstanceProperties.SpecVersion = questionRuleInstanceProperties.SpecVersion
	updateQuestionRuleInstanceProperties.PollingInterval = questionRuleInstanceProperties.PollingInterval
	updateQuestionRuleInstanceProperties.Operations = questionRuleInstanceProperties.Operations
	updateQuestionRuleInstanceProperties.Outputs = questionRuleInstanceProperties.Outputs
	updateQuestionRuleInstanceProperties.Question = questionRuleInstanceProperties.Question
	updateQuestionRuleInstanceProperties.Templates = questionRuleInstanceProperties.Templates

	if v, ok := d.GetOk("version"); ok {
		updateQuestionRuleInstanceProperties.Version = v.(int)
	}

	updatedQuestionRuleInstance, err := m.(*ProviderConfiguration).Client.UpdateQuestionRuleInstance(updateQuestionRuleInstanceProperties)

	if err != nil {
		return fmt.Errorf("Failed to update question rule instance: %s", err.Error())
	}

	d.Set("version", updatedQuestionRuleInstance.Version)
	return nil
}

func resourceQuestionRuleInstanceDelete(d *schema.ResourceData, m interface{}) error {
	if err := m.(*ProviderConfiguration).Client.DeleteQuestionRuleInstance(d.Id()); err != nil {
		return fmt.Errorf("Failed to delete question rule instance: %s", err.Error())
	}

	return nil
}

func resourceQuestionRuleInstanceRead(d *schema.ResourceData, m interface{}) error {
	questionRuleInstance, err := m.(*ProviderConfiguration).Client.GetQuestionRuleInstanceByID(d.Id())

	if err != nil {
		return fmt.Errorf("Failed to read existing question rule instance: %s", err.Error())
	}

	d.Set("version", questionRuleInstance.Version)
	return nil
}
