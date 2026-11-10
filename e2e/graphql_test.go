package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

type jupiterOneAccount struct {
	accountID string
	apiKey    string
	region    string
}

// requireDevAccount fails unless credentials for a dev account are set.
func requireDevAccount(t *testing.T) jupiterOneAccount {
	t.Helper()

	account := jupiterOneAccount{
		accountID: os.Getenv("JUPITERONE_ACCOUNT_ID"),
		apiKey:    os.Getenv("JUPITERONE_API_KEY"),
		region:    os.Getenv("JUPITERONE_REGION"),
	}
	if account.accountID == "" || account.apiKey == "" || account.region == "" {
		t.Fatal("set JUPITERONE_ACCOUNT_ID, JUPITERONE_API_KEY and JUPITERONE_REGION=dev to run the e2e tests")
	}
	if account.region != "dev" {
		t.Fatalf("refusing to run: JUPITERONE_REGION must be dev, got %q", account.region)
	}

	return account
}

type graphQLClient struct {
	account jupiterOneAccount
	http    *http.Client
}

func newGraphQLClient(account jupiterOneAccount) *graphQLClient {
	return &graphQLClient{account: account, http: &http.Client{Timeout: time.Minute}}
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors"`
}

func (c *graphQLClient) do(query string, variables map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return fmt.Errorf("encode graphql request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://graphql.%s.jupiterone.io/", c.account.region), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("LifeOmic-Account", c.account.accountID)
	req.Header.Set("Authorization", "Bearer "+c.account.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send graphql request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read graphql response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("graphql request failed with HTTP %d: %s", resp.StatusCode, raw)
	}

	var decoded graphQLResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	if len(decoded.Errors) > 0 {
		return fmt.Errorf("graphql errors: %+v", decoded.Errors)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(decoded.Data, out); err != nil {
		return fmt.Errorf("decode graphql data: %w", err)
	}

	return nil
}

type idItem struct {
	ID string `json:"id"`
}

type controlSnapshot struct {
	Control struct {
		ID           string   `json:"id"`
		State        string   `json:"state"`
		ControlTests []idItem `json:"controlTests"`
	} `json:"control"`
	ControlRequirements struct {
		Items []idItem `json:"items"`
	} `json:"controlRequirements"`
}

func (c *graphQLClient) control(id string) (controlSnapshot, error) {
	var out controlSnapshot
	err := c.do(`query($id: ID!) {
  control(id: $id) { id state controlTests { id } }
  controlRequirements(input: {controlId: $id}) { items { id } }
}`, map[string]any{"id": id}, &out)
	return out, err
}

func (c *graphQLClient) importFrameworkJSON(name string, framework map[string]any) error {
	return c.do(`mutation($input: ImportFrameworkInput!) { importFramework(input: $input) { success } }`,
		map[string]any{"input": map[string]any{"name": name, "source": "JSON", "frameworkJson": framework}}, nil)
}

type importJob struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (c *graphQLClient) inProgressImportJob(name string) (*importJob, error) {
	var out struct {
		InProgressImportJobs []importJob `json:"inProgressImportJobs"`
	}
	if err := c.do(`{ inProgressImportJobs { name status message } }`, nil, &out); err != nil {
		return nil, err
	}
	for i := range out.InProgressImportJobs {
		if out.InProgressImportJobs[i].Name == name {
			return &out.InProgressImportJobs[i], nil
		}
	}
	return nil, nil
}

func (c *graphQLClient) frameworkIDByName(name string) (string, error) {
	var cursor *string
	for {
		var out struct {
			ControlFrameworks struct {
				Items []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"items"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"controlFrameworks"`
		}
		err := c.do(`query($cursor: String) {
  controlFrameworks(input: {cursor: $cursor}) { items { id name } pageInfo { hasNextPage endCursor } }
}`, map[string]any{"cursor": cursor}, &out)
		if err != nil {
			return "", err
		}
		for _, item := range out.ControlFrameworks.Items {
			if item.Name == name {
				return item.ID, nil
			}
		}
		if !out.ControlFrameworks.PageInfo.HasNextPage {
			return "", nil
		}
		next := out.ControlFrameworks.PageInfo.EndCursor
		cursor = &next
	}
}

type frameworkControl struct {
	ID           string   `json:"id"`
	SourceID     string   `json:"sourceId"`
	State        string   `json:"state"`
	ControlTests []idItem `json:"controlTests"`
}

type frameworkRequirement struct {
	ID       string `json:"id"`
	Controls struct {
		Items []frameworkControl `json:"items"`
	} `json:"controls"`
}

func (c *graphQLClient) frameworkRequirements(frameworkID string) ([]frameworkRequirement, error) {
	var out struct {
		ControlFramework struct {
			Requirements []frameworkRequirement `json:"requirements"`
		} `json:"controlFramework"`
	}
	err := c.do(`query($id: ID!) {
  controlFramework(id: $id) {
    requirements { id controls { items { id sourceId state controlTests { id } } } }
  }
}`, map[string]any{"id": frameworkID}, &out)
	return out.ControlFramework.Requirements, err
}

func (c *graphQLClient) deleteControl(id string) error {
	return c.do(`mutation($id: String!) { deleteControl(input: {id: $id}) { success } }`, map[string]any{"id": id}, nil)
}

func (c *graphQLClient) deleteFramework(id string) error {
	return c.do(`mutation($id: String!) { deleteFramework(input: {id: $id}) { success } }`, map[string]any{"id": id}, nil)
}
