package jupiterone_client

import (
	"log"
	"net/http"
	"time"

	"github.com/machinebox/graphql"
)

const DefaultRegion string = "us"

type JupiterOneClientConfig struct {
	APIKey     string
	AccountID  string
	Region     string
	HTTPClient *http.Client
}

type JupiterOneClient struct {
	apiKey, accountID string
	graphqlClient     *graphql.Client
	RetryTimeout      time.Duration
}

func (c *JupiterOneClientConfig) getRegion() string {
	region := c.Region

	if region == "" {
		region = DefaultRegion
	}

	log.Printf("[info] Utilizing region: %s", region)
	return region
}

func (c *JupiterOneClientConfig) getGraphQLEndpoint() string {
	return "https://api." + c.getRegion() + ".jupiterone.io/graphql"
}

func (c *JupiterOneClientConfig) Client() (*JupiterOneClient, error) {
	endpoint := c.getGraphQLEndpoint()

	var client *graphql.Client

	if c.HTTPClient != nil {
		client = graphql.NewClient(endpoint, graphql.WithHTTPClient(c.HTTPClient))
	} else {
		client = graphql.NewClient(endpoint)
	}

	jupiterOneClient := &JupiterOneClient{
		apiKey:        c.APIKey,
		accountID:     c.AccountID,
		graphqlClient: client,
		RetryTimeout:  time.Duration(60 * time.Second),
	}

	return jupiterOneClient, nil
}

func (c *JupiterOneClient) prepareRequest(query string) *graphql.Request {
	req := graphql.NewRequest(query)

	req.Header.Set("LifeOmic-Account", c.accountID)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return req
}
