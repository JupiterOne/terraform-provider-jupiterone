package jupiterone

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
	"gopkg.in/dnaeon/go-vcr.v3/cassette"
	"gopkg.in/dnaeon/go-vcr.v3/recorder"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
func testAccProtoV6ProviderFactories(qlient graphql.Client) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"jupiterone": providerserver.NewProtocol6WithError(NewTestProvider(qlient)()),
	}
}

func NewTestProvider(qlient graphql.Client) func() provider.Provider {
	return func() provider.Provider {
		return &JupiterOneProvider{
			version: "test",
			Qlient:  qlient,
		}
	}
}

func stripHeadersFromCassetteInteraction(i *cassette.Interaction) error {
	i.Request.Headers.Del("LifeOmic-Account")
	i.Request.Headers.Del("Authorization")
	i.Response.Headers.Del("Date")
	i.Response.Headers.Del("Via")
	i.Response.Headers.Del("X-Amz-Apigw-Id")
	i.Response.Headers.Del("X-Amz-Cf-Id")
	i.Response.Headers.Del("X-Amz-Cf-Pop")
	i.Response.Headers.Del("X-Cache")

	return nil
}

func setupCassettes(name string) (*recorder.Recorder, func(t *testing.T)) {
	rec, err := recorder.NewWithOptions(&recorder.Options{
		CassetteName:       fmt.Sprintf("cassettes/%s", name),
		Mode:               recorder.ModeRecordOnce,
		RealTransport:      cleanhttp.DefaultTransport(),
		SkipRequestLatency: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	rec.SetMatcher(func(req *http.Request, c cassette.Request) bool {
		// ignore hostname prefixes and URI paths on replays
		return req.Method == c.Method && strings.HasSuffix(req.Host, "jupiterone.io")
	})

	rec.AddHook(stripHeadersFromCassetteInteraction, recorder.BeforeSaveHook)

	cleanup := func(t *testing.T) {
		_ = rec.Stop()
	}
	return rec, cleanup
}

// setupTestClients creates clients to be used during tests
//
//   - recorderClient: uses a go-vcr recorder for replaying API responses
//   - directClient: nil when not recording, for sending requests directly J1 for
//     requests that verify the state during recording, but don't need to be
//     repeated during replays.
func setupTestClients(ctx context.Context, t *testing.T) (recordingClient graphql.Client, directClient graphql.Client, cleanup func(t *testing.T)) {
	var recorder *recorder.Recorder

	recorder, cleanup = setupCassettes(t.Name())

	recordingClient = client.NewQlientFromEnv(ctx, recorder)

	if recorder.IsRecording() {
		directClient = client.NewQlientFromEnv(ctx, nil)
	}

	return
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("JUPITERONE_API_KEY"); v == "" {
		t.Fatal("JUPITERONE_API_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("JUPITERONE_ACCOUNT_ID"); v == "" {
		t.Fatal("JUPITERONE_ACCOUNT_ID must be set for acceptance tests")
	}
	if v := os.Getenv("JUPITERONE_REGION"); v == "" {
		t.Fatal("JUPITERONE_REGION must be set for acceptance tests")
	}
}
