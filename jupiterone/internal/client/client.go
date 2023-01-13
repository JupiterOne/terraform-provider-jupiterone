package client

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/machinebox/graphql"
)

const DefaultRegion string = "us"

type JupiterOneClientConfig struct {
	APIKey    string
	AccountID string
	Region    string
	// Client is mostly used to inject the `go-vcr` transport recorder
	// for testing
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
		RetryTimeout:  time.Minute,
	}

	return jupiterOneClient, nil
}

// NewClientFromEnv configures the J1 client itself from the environment
// variables for use in testing.
func NewClientFromEnv(ctx context.Context, client *http.Client) (*JupiterOneClient, error) {
	config := JupiterOneClientConfig{
		APIKey:     os.Getenv("JUPITERONE_API_KEY"),
		AccountID:  os.Getenv("JUPITERONE_ACCOUNT_ID"),
		Region:     os.Getenv("JUPITERONE_REGION"),
		HTTPClient: client,
	}

	return config.Client()
}

func (c *JupiterOneClient) prepareRequest(query string) *graphql.Request {
	req := graphql.NewRequest(query)

	req.Header.Set("LifeOmic-Account", c.accountID)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	return req
}
