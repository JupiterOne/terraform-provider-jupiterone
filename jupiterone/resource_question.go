package jupiterone

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mitchellh/mapstructure"

	jupiterone "github.com/jupiterone/terraform-provider-jupiterone/jupiterone_client"
)

func resourceQuestion() *schema.Resource {
	return &schema.Resource{
		Create: resourceQuestionCreate,
		Read:   resourceQuestionRead,
		Update: resourceQuestionUpdate,
		Delete: resourceQuestionDelete,

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

func buildQuestionTagList(terraformTagList []interface{}) []string {
	tagList := make([]string, len(terraformTagList))

	for i, tag := range terraformTagList {
		tagList[i] = tag.(string)
	}

	return tagList
}

func buildQuestionQueryList(terraformQuestionQueryList []interface{}) (*[]jupiterone.QuestionQuery, error) {
	questionQueryList := make([]jupiterone.QuestionQuery, len(terraformQuestionQueryList))

	for i, terraformQuestionQuery := range terraformQuestionQueryList {
		var query jupiterone.QuestionQuery

		if err := mapstructure.Decode(terraformQuestionQuery, &query); err != nil {
			return nil, err
		}

		questionQueryList[i] = query
	}

	return &questionQueryList, nil
}

func buildQuestionComplianceMetaDataList(terraformComplianceList []interface{}) (*[]jupiterone.QuestionComplianceMetaData, error) {
	complianceMetaDataList := make([]jupiterone.QuestionComplianceMetaData, len(terraformComplianceList))

	for i, terraformComplianceMetaData := range terraformComplianceList {
		var complianceMetaData jupiterone.QuestionComplianceMetaData

		if err := mapstructure.Decode(terraformComplianceMetaData, &complianceMetaData); err != nil {
			return nil, err
		}

		complianceMetaDataList[i] = complianceMetaData
	}

	return &complianceMetaDataList, nil
}

func buildQuestionProperties(d *schema.ResourceData) (*jupiterone.QuestionProperties, error) {
	var question jupiterone.QuestionProperties

	if v, ok := d.GetOk("title"); ok {
		question.Title = v.(string)
	}

	if v, ok := d.GetOk("description"); ok {
		question.Description = v.(string)
	}

	if v, ok := d.GetOk("tags"); ok {
		question.Tags = buildQuestionTagList(v.([]interface{}))
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

func resourceQuestionCreate(d *schema.ResourceData, m interface{}) error {
	questionProperties, err := buildQuestionProperties(d)
	if err != nil {
		return fmt.Errorf("Failed to build question: %s", err.Error())
	}

	createdQuestion, err := m.(*ProviderConfiguration).Client.CreateQuestion(*questionProperties)
	if err != nil {
		return fmt.Errorf("Failed to create question: %s", err.Error())
	}

	d.SetId(createdQuestion.Id)

	return nil
}

func resourceQuestionRead(d *schema.ResourceData, m interface{}) error {
	if _, err := m.(*ProviderConfiguration).Client.GetQuestion(d.Id()); err != nil {
		return fmt.Errorf("Failed to read existing question: %s", err.Error())
	}

	return nil
}

func resourceQuestionUpdate(d *schema.ResourceData, m interface{}) error {
	questionProperties, err := buildQuestionProperties(d)
	if err != nil {
		return fmt.Errorf("Failed to build question: %s", err.Error())
	}

	if _, err := m.(*ProviderConfiguration).Client.UpdateQuestion(d.Id(), *questionProperties); err != nil {
		return fmt.Errorf("Failed to update question: %s", err.Error())
	}

	return nil
}

func resourceQuestionDelete(d *schema.ResourceData, m interface{}) error {
	if err := m.(*ProviderConfiguration).Client.DeleteQuestion(d.Id()); err != nil {
		return fmt.Errorf("Failed to delete question: %s", err.Error())
	}

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
