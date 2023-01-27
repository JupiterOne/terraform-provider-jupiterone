package client

import (
	"context"

	"github.com/mitchellh/mapstructure"
)

type QuestionQuery struct {
	Query   string `json:"query"`
	Version string `json:"version"`
	Name    string `json:"name"`
}

type QuestionComplianceMetaData struct {
	Standard     string   `json:"standard"`
	Requirements []string `json:"requirements"`
	Controls     []string `json:"controls"`
}

type Question struct {
	Id          string                       `json:"id,omitempty"`
	Title       string                       `json:"title"`
	Description string                       `json:"description"`
	Tags        []string                     `json:"tags"`
	Queries     []QuestionQuery              `json:"queries"`
	Compliance  []QuestionComplianceMetaData `json:"compliance"`
}

func (c *JupiterOneClient) GetQuestion(id string) (*Question, error) {
	req := c.prepareRequest(`
		query GetQuestionById ($id: ID!) {
			question(id: $id) {
				id
				title
				description
				queries {
					name
					query
					version
				}
				tags
				compliance {
					type
					details {
						name
						description
					}
				}
				accountId
				integrationDefinitionId
			}
		}
	`)

	req.Var("id", id)

	var respData map[string]interface{}

	if err := c.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	var question Question

	if err := mapstructure.Decode(respData["question"], &question); err != nil {
		return nil, err
	}

	return &question, nil
}

func (c *JupiterOneClient) CreateQuestion(question *Question) (*Question, error) {
	req := c.prepareRequest(`
		mutation CreateQuestion($question: CreateQuestionInput!) {
			createQuestion(question: $question) {
				id
				title
				description
				queries {
					name
					query
					version
				}
				tags
				variables {
					name
					required
					default
				}
				compliance {
					type
					details {
						name
						description
					}
				}
			}
		}
	`)

	req.Var("question", question)

	var respData map[string]interface{}
	var created *Question
	if err := c.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	if err := mapstructure.Decode(respData["createQuestion"], &created); err != nil {
		return nil, err
	}

	return created, nil
}

func (c *JupiterOneClient) UpdateQuestion(id string, q *Question) (*Question, error) {
	req := c.prepareRequest(`
		mutation UpdateQuestion ($id: ID!, $update: QuestionUpdate!) {
			updateQuestion(id: $id, update: $update) {
				id
				title
				description
				queries {
					name
					query
					version
				}
				tags
				variables {
					name
					required
					default
				}
				compliance {
					type
					details {
						name
						description
					}
				}
			}
		}
	`)

	req.Var("id", id)
	req.Var("update", q)

	var respData map[string]interface{}
	var updated *Question
	if err := c.graphqlClient.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	if err := mapstructure.Decode(respData["updateQuestion"], &updated); err != nil {
		return nil, err
	}

	return q, nil
}

func (c *JupiterOneClient) DeleteQuestion(id string) error {
	req := c.prepareRequest(`
		mutation DeleteQuestion($id: ID!) {
			deleteQuestion(id: $id) {
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
