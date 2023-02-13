package client

//go:generate go run github.com/Khan/genqlient

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Khan/genqlient/graphql"
	genql "github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
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

type jupiterOneTransport struct {
	apiKey, accountID string
	base              http.RoundTripper
}

func (t *jupiterOneTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("LifeOmic-Account", t.accountID)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)
	return t.base.RoundTrip(req)
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

// NewQlientFromEnv configures the J1 client itself from the environment
// variables for use in testing.
func NewQlientFromEnv(ctx context.Context, client *http.Client) graphql.Client {
	config := JupiterOneClientConfig{
		APIKey:     os.Getenv("JUPITERONE_API_KEY"),
		AccountID:  os.Getenv("JUPITERONE_ACCOUNT_ID"),
		Region:     os.Getenv("JUPITERONE_REGION"),
		HTTPClient: client,
	}

	return config.Qlient()
}

func (c *JupiterOneClientConfig) Qlient() graphql.Client {
	endpoint := c.getGraphQLEndpoint()

	transport := http.DefaultTransport
	httpClient := &http.Client{}
	if c.HTTPClient != nil {
		httpClient = c.HTTPClient
		transport = c.HTTPClient.Transport
	}

	transport = &jupiterOneTransport{apiKey: c.APIKey, accountID: c.AccountID, base: transport}
	transport = logging.NewLoggingHTTPTransport(transport)
	httpClient.Transport = transport

	client := genql.NewClient(endpoint, httpClient)

	return client
}
