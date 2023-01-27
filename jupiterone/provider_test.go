package jupiterone

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
func testAccProtoV6ProviderFactories(j1Client *client.JupiterOneClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"jupiterone": providerserver.NewProtocol6WithError(NewTestProvider(j1Client)()),
	}
}

func NewTestProvider(j1Client *client.JupiterOneClient) func() provider.Provider {
	return func() provider.Provider {
		return &JupiterOneProvider{
			version: "test",
			Client:  j1Client,
		}
	}
}

func isRecording() bool {
	return os.Getenv("RECORD") == "true"
}

func isReplaying() bool {
	return os.Getenv("RECORD") == "false"
}

// Ensure that the URL that we store in cassettes is always consistent regardless
// of what region is specified.
func normalizeURL(u *url.URL) *url.URL {
	u.Host = "api.us.jupiterone.io"
	return u
}

func stripHeadersFromCassetteInteraction(i *cassette.Interaction) {
	i.Request.Headers.Del("LifeOmic-Account")
	i.Request.Headers.Del("Authorization")
	i.Response.Headers.Del("Date")
	i.Response.Headers.Del("Via")
	i.Response.Headers.Del("X-Amz-Apigw-Id")
	i.Response.Headers.Del("X-Amz-Cf-Id")
	i.Response.Headers.Del("X-Amz-Cf-Pop")
	i.Response.Headers.Del("X-Cache")
}

func setupCassettes(name string) (*recorder.Recorder, func(t *testing.T)) {
	var mode recorder.Mode
	if isRecording() {
		mode = recorder.ModeRecording
	} else if isReplaying() {
		mode = recorder.ModeReplaying
	} else {
		mode = recorder.ModeDisabled
	}

	rec, err := recorder.NewAsMode(fmt.Sprintf("cassettes/%s", name), mode, nil)
	if err != nil {
		log.Fatal(err)
	}

	rec.SetMatcher(func(r *http.Request, i cassette.Request) bool {
		return r.Method == i.Method && normalizeURL(r.URL).String() == i.URL
	})

	rec.AddFilter(func(i *cassette.Interaction) error {
		u, err := url.Parse(i.URL)
		if err != nil {
			return err
		}

		i.URL = normalizeURL(u).String()
		stripHeadersFromCassetteInteraction(i)
		return nil
	})
	cleanup := func(t *testing.T) {
		_ = rec.Stop()
	}
	return rec, cleanup
}

func testAccPreCheck(t *testing.T) {
	if isReplaying() {
		return
	}
	if v := os.Getenv("JUPITERONE_API_KEY"); v == "" {
		t.Fatal("JUPITERONE_API_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("JUPITERONE_ACCOUNT_ID"); v == "" {
		t.Fatal("JUPITERONE_ACCOUNT_ID must be set for acceptance tests")
	}
}
