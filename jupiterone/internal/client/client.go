package client

//go:generate go run github.com/Khan/genqlient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Khan/genqlient/graphql"
	genql "github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/logging"
)

const DefaultRegion string = "us"

var (
	lastNon429Response time.Time
	timestampMutex     sync.Mutex
)

func updateLastNon429Response() {
	timestampMutex.Lock()
	defer timestampMutex.Unlock()
	lastNon429Response = time.Now()
}

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

// RetryTransport is a custom RoundTripper that adds retry logic with backoff.
type RetryTransport struct {
	Transport  http.RoundTripper
	MaxRetries int
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	ctx := req.Context()

	// We need to keep a copy of the body because each request it gets consumed
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %v", err)
		}
	}

	for i := 0; i >= 0; i++ {
		// Setting the body for the request
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		resp, _ = rt.Transport.RoundTrip(req)

		if resp.StatusCode != http.StatusTooManyRequests {
			updateLastNon429Response()
			return resp, nil
		}

		// If this is not the first try, and the lastNon429Response
		// was more than 90 seconds ago, we should break out of the loop
		// and return the last response.
		timestampMutex.Lock()
		if i > 0 && time.Since(lastNon429Response) > 90*time.Second {
			timestampMutex.Unlock()
			tflog.Warn(ctx, "Not going to retry, we haven't got a non 429 in a while")
			return resp, nil
		}
		timestampMutex.Unlock()

		tflog.Debug(ctx, "Retrying after getting a 429 response")

		// Calculate the backoff time using exponential backoff with jitter.
		backoff := rt.MinBackoff * time.Duration(math.Pow(2, float64(i)))
		jitter := time.Duration(rand.Int63n(int64(rt.MinBackoff)))
		sleepDuration := backoff + jitter

		// Ensure we do not exceed the maximum backoff time.
		if sleepDuration > rt.MaxBackoff {
			sleepDuration = rt.MaxBackoff
		}

		// Convert duration to seconds
		var backoffSeconds = int(sleepDuration.Seconds())

		tflog.Debug(ctx, "Backoff info", map[string]interface{}{"sleepDurationSeconds": backoffSeconds, "retryCount": i})

		time.Sleep(sleepDuration)
	}

	return resp, fmt.Errorf("after %d attempts, last status: %s", rt.MaxRetries, strconv.Itoa(resp.StatusCode))
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

	httpClient.Transport = &RetryTransport{
		Transport:  httpClient.Transport,
		MaxRetries: 50,
		MinBackoff: 15 * time.Second, // Initial backoff duration
		MaxBackoff: 60 * time.Second, // Maximum backoff duration
	}

	client := genql.NewClient(endpoint, httpClient)

	return client
}
