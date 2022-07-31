package jupiterone

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
	"github.com/mitchellh/mapstructure"
)

func resourceQuestion() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceQuestionCreate,
		ReadContext:   resourceQuestionRead,
		UpdateContext: resourceQuestionUpdate,
		DeleteContext: resourceQuestionDelete,
		Schema: map[string]*schema.Schema{
			"title": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The title of the question",
			},
			"description": {
				Type:     schema.TypeString,
				Required: true,
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"query": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					// from resource_question_rule_instance.go
					Schema: getQuestionQuerySchema(),
				},
			},
			"compliance": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: getQuestionComplianceSchema(),
				},
			},
		},
	}
}

func buildQuestionQueryList(terraformQuestionQueryList []interface{}) (*[]client.QuestionQuery, error) {
	questionQueryList := make([]client.QuestionQuery, len(terraformQuestionQueryList))

	for i, terraformQuestionQuery := range terraformQuestionQueryList {
		var query client.QuestionQuery

		if err := mapstructure.Decode(terraformQuestionQuery, &query); err != nil {
			return nil, err
		}

		query.Query = removeCRFromString(query.Query)

		questionQueryList[i] = query
	}

	return &questionQueryList, nil
}

func buildQuestionComplianceMetaDataList(terraformComplianceList []interface{}) (*[]client.QuestionComplianceMetaData, error) {
	complianceMetaDataList := make([]client.QuestionComplianceMetaData, len(terraformComplianceList))

	for i, terraformComplianceMetaData := range terraformComplianceList {
		var complianceMetaData client.QuestionComplianceMetaData

		if err := mapstructure.Decode(terraformComplianceMetaData, &complianceMetaData); err != nil {
			return nil, err
		}

		complianceMetaDataList[i] = complianceMetaData
	}

	return &complianceMetaDataList, nil
}

func buildQuestionProperties(d *schema.ResourceData) (*client.QuestionProperties, error) {
	var question client.QuestionProperties

	if v, ok := d.GetOk("title"); ok {
		question.Title = v.(string)
	}

	if v, ok := d.GetOk("description"); ok {
		question.Description = v.(string)
	}

	if v, ok := d.GetOk("tags"); ok {
		question.Tags = interfaceSliceToStringSlice(v.([]interface{}))
	}

	if v, ok := d.GetOk("query"); ok {
		queries, err := buildQuestionQueryList(v.([]interface{}))

		if err != nil {
			return nil, err
		}

		question.Queries = *queries
	}

	if v, ok := d.GetOk("compliance"); ok {
		complianceList, err := buildQuestionComplianceMetaDataList(v.([]interface{}))

		if err != nil {
			return nil, err
		}

		question.Compliance = *complianceList
	}

	return &question, nil
}

func resourceQuestionCreate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	questionProperties, err := buildQuestionProperties(d)
	if err != nil {
		return diag.Errorf("failed to build question: %s", err.Error())
	}

	createdQuestion, err := m.(*ProviderConfiguration).Client.CreateQuestion(*questionProperties)
	if err != nil {
		return diag.Errorf("failed to create question: %s", err.Error())
	}

	d.SetId(createdQuestion.Id)

	return nil
}

func resourceQuestionRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	question, err := m.(*ProviderConfiguration).Client.GetQuestion(d.Id())
	if err != nil {
		return diag.Errorf("failed to read existing question: %s", err.Error())
	}

	d.SetId(question.Id)
	return nil
}

func resourceQuestionUpdate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	questionProperties, err := buildQuestionProperties(d)
	if err != nil {
		return diag.Errorf("failed to build question: %s", err.Error())
	}

	if _, err := m.(*ProviderConfiguration).Client.UpdateQuestion(d.Id(), *questionProperties); err != nil {
		return diag.Errorf("failed to update question: %s", err.Error())
	}

	return nil
}

func resourceQuestionDelete(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	if err := m.(*ProviderConfiguration).Client.DeleteQuestion(d.Id()); err != nil {
		return diag.Errorf("failed to delete question: %s", err.Error())
	}

	d.SetId("")
	return nil
}

func getQuestionComplianceSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"standard": {
			Type:     schema.TypeString,
			Required: true,
		},
		"requirements": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
		"controls": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
		},
	}
}
