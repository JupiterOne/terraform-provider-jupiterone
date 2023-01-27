package client

import (
	"context"

	"github.com/machinebox/graphql"
)

type QuestionRuleInstance struct {
	Id              string            `json:"id,omitempty"`
	AccountId       string            `json:"accountId,omitempty"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Version         int               `json:"version,omitempty"`
	SpecVersion     int               `json:"specVersion,omitempty"`
	Latest          bool              `json:"latest,omitempty"`
	Deleted         bool              `json:"deleted,omitempty"`
	Type            string            `json:"type,omitempty"`
	PollingInterval string            `json:"pollingInterval"`
	Templates       map[string]string `json:"templates"`
	// Question: TODO: make into structs
	Question     map[string][]map[string]string `json:"question,omitempty"`
	QuestionId   string                         `json:"questionId,omitempty"`
	QuestionName string                         `json:"questionName,omitempty"`
	Operations   []RuleOperation                `json:"operations"`
	Outputs      []string                       `json:"outputs"`
	Tags         []string                       `json:"tags"`
}

type RuleOperation struct {
	When    map[string]interface{}   `json:"when,omitempty"`
	Actions []map[string]interface{} `json:"actions"`
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
				tags
			}
		}
	`)

	req.Var("id", id)

	resp := struct {
		QuestionRuleInstance QuestionRuleInstance `json:"questionRuleInstance"`
	}{
		QuestionRuleInstance: QuestionRuleInstance{},
	}

	if err := c.graphqlClient.Run(context.Background(), req, &resp); err != nil {
		return nil, err
	}

	return &resp.QuestionRuleInstance, nil
}

func (c *JupiterOneClient) CreateQuestionRuleInstance(questionRuleInstance *QuestionRuleInstance) (*QuestionRuleInstance, error) {
	var req *graphql.Request
	if questionRuleInstance.QuestionId != "" || questionRuleInstance.QuestionName != "" {
		req = c.prepareRequest(`
		mutation CreateQuestionRuleInstance ($instance: CreateReferencedQuestionRuleInstanceInput!) {
				createQuestionRuleInstance: createReferencedQuestionRuleInstance (
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
					questionId
					questionName
					operations {
						when
						actions
					}
					outputs
					tags
				}
			}
		`)
	} else {
		req = c.prepareRequest(`
			mutation CreateQuestionRuleInstance ($instance: CreateInlineQuestionRuleInstanceInput!) {
				createQuestionRuleInstance: createInlineQuestionRuleInstance (
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
					tags
				}
			}
		`)
	}
	req.Var("instance", questionRuleInstance)

	resp := struct {
		CreateQuestionRuleInstance *QuestionRuleInstance `json:"createQuestionRuleInstance"`
	}{
		CreateQuestionRuleInstance: &QuestionRuleInstance{},
	}

	err := c.graphqlClient.Run(context.Background(), req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.CreateQuestionRuleInstance, nil
}

func (c *JupiterOneClient) UpdateQuestionRuleInstance(instance *QuestionRuleInstance) (*QuestionRuleInstance, error) {
	var req *graphql.Request
	if instance.QuestionId != "" || instance.QuestionName != "" {
		req = c.prepareRequest(`
		mutation UpdateQuestionRuleInstance ($instance: UpdateReferencedQuestionRuleInstanceInput!) {
				updateQuestionRuleInstance: updateReferencedQuestionRuleInstance (
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
					questionId
					questionName
					operations {
						when
						actions
					}
					outputs
					tags
				}
			}
			`)
	} else {
		req = c.prepareRequest(`
		mutation UpdateQuestionRuleInstance ($instance: UpdateInlineQuestionRuleInstanceInput!) {
				updateQuestionRuleInstance: updateInlineQuestionRuleInstance (
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
					tags
				}
			}
		`)
	}

	req.Var("instance", instance)
	resp := struct {
		UpdateQuestionRuleInstance *QuestionRuleInstance `json:"updateQuestionRuleInstance"`
	}{
		UpdateQuestionRuleInstance: &QuestionRuleInstance{},
	}

	err := c.graphqlClient.Run(context.Background(), req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.UpdateQuestionRuleInstance, nil

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
