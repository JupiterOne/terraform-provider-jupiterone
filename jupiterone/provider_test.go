package jupiterone

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/logging"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/jonboulle/clockwork"
	jupiterone "github.com/jupiterone/terraform-provider-jupiterone/jupiterone_client"
)

func isRecording() bool {
	return os.Getenv("RECORD") == "true"
}

func isReplaying() bool {
	return os.Getenv("RECORD") == "false"
}

func setClock(t *testing.T) clockwork.FakeClock {
	os.MkdirAll("cassettes", 0755)
	f, err := os.Create(fmt.Sprintf("cassettes/%s.freeze", t.Name()))
	if err != nil {
		t.Fatalf("Could not set clock: %v", err)
	}
	defer f.Close()
	now := clockwork.NewRealClock().Now()
	f.WriteString(now.Format(time.RFC3339Nano))
	return clockwork.NewFakeClockAt(now)
}

func restoreClock(t *testing.T) clockwork.FakeClock {
	data, err := ioutil.ReadFile(fmt.Sprintf("cassettes/%s.freeze", t.Name()))
	if err != nil {
		t.Logf("Could not load clock: %v", err)
		return setClock(t)
	}
	now, err := time.Parse(time.RFC3339Nano, string(data))
	if err != nil {
		t.Fatalf("Could not parse clock date: %v", err)
	}
	return clockwork.NewFakeClockAt(now)
}

func testClock(t *testing.T) clockwork.FakeClock {
	if isRecording() {
		return setClock(t)
	} else if isReplaying() {
		return restoreClock(t)
	}
	// do not set or restore frozen time
	return clockwork.NewFakeClockAt(clockwork.NewRealClock().Now())
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

func initAccProvider(t *testing.T) (*schema.Provider, func(t *testing.T)) {
	var mode recorder.Mode
	if isRecording() {
		mode = recorder.ModeRecording
	} else if isReplaying() {
		mode = recorder.ModeReplaying
	} else {
		mode = recorder.ModeDisabled
	}

	rec, err := recorder.NewAsMode(fmt.Sprintf("cassettes/%s", t.Name()), mode, nil)
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

	p := Provider().(*schema.Provider)
	p.ConfigureFunc = testProviderConfigure(rec)

	cleanup := func(t *testing.T) {
		rec.Stop()
	}
	return p, cleanup
}

func testProviderConfigure(recorder *recorder.Recorder) schema.ConfigureFunc {
	return func(d *schema.ResourceData) (interface{}, error) {
		testHTTPClient := cleanhttp.DefaultClient()
		testHTTPClient.Transport = logging.NewTransport("JupiterOne", recorder)

		config := jupiterone.JupiterOneClientConfig{
			APIKey:     d.Get("api_key").(string),
			AccountID:  d.Get("account_id").(string),
			Region:     d.Get("region").(string),
			HTTPClient: testHTTPClient,
		}

		client, err := config.Client()

		if err != nil {
			return nil, fmt.Errorf("Failed to create JupiterOne client in provider configuration: %s", err.Error())
		}

		return &ProviderConfiguration{
			Client: client,
		}, nil
	}
}

func testAccProviders(t *testing.T) (map[string]terraform.ResourceProvider, func(t *testing.T)) {
	provider, cleanup := initAccProvider(t)
	return map[string]terraform.ResourceProvider{
		"jupiterone": provider,
	}, cleanup
}

func testAccProvider(t *testing.T, accProviders map[string]terraform.ResourceProvider) *schema.Provider {
	accProvider, ok := accProviders["jupiterone"]
	if !ok {
		t.Fatal("Could not find jupiterone provider")
	}
	return accProvider.(*schema.Provider)
}

func TestProvider(t *testing.T) {
	accProvider, cleanup := initAccProvider(t)
	defer cleanup(t)

	if err := accProvider.InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvider_impl(t *testing.T) {
	var _ terraform.ResourceProvider = Provider()
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
