package client

//go:generate go run github.com/Khan/genqlient

import (
	"context"
	"net/http"
	"os"

	"github.com/Khan/genqlient/graphql"
	genql "github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
)

const DefaultRegion string = "us"

type JupiterOneClientConfig struct {
	APIKey    string
	AccountID string
	Region    string
	// RoundTripper is mostly used to inject the `go-vcr` transport recorder
	// for testing
	RoundTripper http.RoundTripper
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

func (c *JupiterOneClientConfig) getRegion(ctx context.Context) string {
	region := c.Region

	if region == "" {
		region = DefaultRegion
	}

	tflog.Info(ctx, "Utilizing region", map[string]interface{}{"region": region})
	return region
}

func (c *JupiterOneClientConfig) getGraphQLEndpoint(ctx context.Context) string {
	return "https://graphql." + c.getRegion(ctx) + ".jupiterone.io/"
}

// NewQlientFromEnv configures the J1 client itself from the environment
// variables for use in testing.
func NewQlientFromEnv(ctx context.Context, transport http.RoundTripper) graphql.Client {
	config := JupiterOneClientConfig{
		APIKey:       os.Getenv("JUPITERONE_API_KEY"),
		AccountID:    os.Getenv("JUPITERONE_ACCOUNT_ID"),
		Region:       os.Getenv("JUPITERONE_REGION"),
		RoundTripper: transport,
	}

	return config.Qlient(ctx)
}

func (c *JupiterOneClientConfig) Qlient(ctx context.Context) graphql.Client {
	endpoint := c.getGraphQLEndpoint(ctx)

	httpClient := cleanhttp.DefaultClient()
	if c.RoundTripper != nil {
		httpClient.Transport = c.RoundTripper
	}

	httpClient.Transport = &jupiterOneTransport{apiKey: c.APIKey, accountID: c.AccountID, base: httpClient.Transport}
	httpClient.Transport = logging.NewLoggingHTTPTransport(httpClient.Transport)

	client := genql.NewClient(endpoint, httpClient)

	return client
}
