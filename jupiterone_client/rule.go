package jupiterone_client

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mitchellh/mapstructure"
)

type RuleQuestion struct {
	Queries []QuestionQuery `json:"queries"`
}

type RuleOperation struct {
	When    []map[string]interface{} `json:"when"`
	Actions []string                 `json:"actions"`
}

type QuestionRuleInstance struct {
	BaseQuestionRuleInstanceProperties
	Id        string `json:"id"`
	AccountId string `json:"accountId"`
	Version   int    `json:"version"`
	Latest    bool   `json:"latest"`
	Deleted   bool   `json:"deleted"`
	Type      string `json:"type"`
}

type UpdateQuestionRuleInstanceProperties struct {
	BaseQuestionRuleInstanceProperties
	Id      string `json:"id"`
	Version int    `json:"version"`
}

type BaseQuestionRuleInstanceProperties struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	SpecVersion     int                    `json:"specVersion"`
	PollingInterval string                 `json:"pollingInterval"`
	Outputs         []string               `json:"outputs"`
	Operations      string                 `json:"operations"`
	Question        RuleQuestion           `json:"question"`
	Templates       map[string]interface{} `json:"templates"`
}

type CreateQuestionRuleInstanceInput struct {
	BaseQuestionRuleInstanceProperties
	Operations []map[string]interface{} `json:"operations"`
}

type UpdateQuestionRuleInstanceInput struct {
	UpdateQuestionRuleInstanceProperties
	Operations []map[string]interface{} `json:"operations"`
}

// GetQuestionRuleInstanceByID - Fetches the QuestionRuleInstance by unique id
func (c *JupiterOneClient) GetQuestionRuleInstanceByID(id string) (*QuestionRuleInstance, error) {
	req := c.prepareRequest(`
		query GetQuestionRuleInstance($id: ID!) {
			questionRuleInstance (id: $id) {
				id
				name
				description
				version
				specVersion
				latest
				pollingInterval
				deleted
				accountId
				type
				templates
				question {
					queries {
						name
						query
						version
					}
				}
				operations {
					when
					actions
				}
				outputs
			}
		}
	`)

	req.Var("id", id)

	var respData map[string]interface{}
	if err := c.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	var decodedQuestionRuleInstance QuestionRuleInstance
	err := mapstructure.Decode(respData["questionRuleInstance"], &decodedQuestionRuleInstance)
	if err != nil {
		return nil, err
	}

	return &decodedQuestionRuleInstance, nil
}

// CreateQuestionRuleInstance - Creates a question rule instance
func (c *JupiterOneClient) CreateQuestionRuleInstance(createQuestionRuleInstanceInput BaseQuestionRuleInstanceProperties) (*QuestionRuleInstance, error) {
	log.Println("Create question rule instance: " + createQuestionRuleInstanceInput.Name)

	req := c.prepareRequest(`
		mutation CreateQuestionRuleInstance ($instance: CreateQuestionRuleInstanceInput!) {
			createQuestionRuleInstance (
				instance: $instance
			) {
				id
				name
				description
				version
				specVersion
				latest
				deleted
				accountId
				type
				pollingInterval
				templates
				question {
					queries {
						name
						query
						version
					}
				}
				operations {
					when
					actions
				}
				outputs
			}
		}
	`)

	var input CreateQuestionRuleInstanceInput
	input.Name = createQuestionRuleInstanceInput.Name
	input.Description = createQuestionRuleInstanceInput.Description
	input.SpecVersion = createQuestionRuleInstanceInput.SpecVersion
	input.PollingInterval = createQuestionRuleInstanceInput.PollingInterval
	input.Outputs = createQuestionRuleInstanceInput.Outputs
	input.Question = createQuestionRuleInstanceInput.Question
	input.Templates = createQuestionRuleInstanceInput.Templates

	var deserializedOperationsMap []map[string]interface{}

	err := json.Unmarshal([]byte(createQuestionRuleInstanceInput.Operations), &deserializedOperationsMap)

	if err != nil {
		return nil, err
	}

	input.Operations = deserializedOperationsMap

	req.Var("instance", input)

	var respData map[string]interface{}

	if err := c.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	var questionRuleInstance QuestionRuleInstance

	if err := mapstructure.Decode(respData["createQuestionRuleInstance"], &questionRuleInstance); err != nil {
		return nil, err
	}

	return &questionRuleInstance, nil
}

func (c *JupiterOneClient) UpdateQuestionRuleInstance(properties UpdateQuestionRuleInstanceProperties) (*QuestionRuleInstance, error) {
	log.Println("Updating question rule instance: " + properties.Name)

	req := c.prepareRequest(`
		mutation UpdateQuestionRuleInstance ($instance: UpdateQuestionRuleInstanceInput!) {
			updateQuestionRuleInstance (
				instance: $instance
			) {
				id
				name
				description
				version
				specVersion
				latest
				deleted
				accountId
				type
				pollingInterval
				templates
				question {
					queries {
						name
						query
						version
					}
				}
				operations {
					when
					actions
				}
				outputs
			}
		}
	`)

	var input UpdateQuestionRuleInstanceInput
	input.Id = properties.Id
	input.Version = properties.Version
	input.Name = properties.Name
	input.Description = properties.Description
	input.SpecVersion = properties.SpecVersion
	input.PollingInterval = properties.PollingInterval
	input.Outputs = properties.Outputs
	input.Question = properties.Question
	input.Templates = properties.Templates

	var deserializedOperationsMap []map[string]interface{}

	err := json.Unmarshal([]byte(properties.Operations), &deserializedOperationsMap)

	if err != nil {
		return nil, err
	}

	input.Operations = deserializedOperationsMap

	req.Var("instance", input)
	var respData map[string]interface{}

	if err := c.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	var questionRuleInstance QuestionRuleInstance

	if err := mapstructure.Decode(respData["updateQuestionRuleInstance"], &questionRuleInstance); err != nil {
		return nil, err
	}

	return &questionRuleInstance, nil
}

func (c *JupiterOneClient) DeleteQuestionRuleInstance(id string) error {
	req := c.prepareRequest(`
		mutation DeleteRuleInstance ($id: ID!) {
			deleteRuleInstance (id: $id) {
				id
			}
	      }
	`)

	req.Var("id", id)

	if err := c.graphqlClient.Run(context.Background(), req, nil); err != nil {
		return err
	}

	return nil
}
