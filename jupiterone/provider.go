package jupiterone

import (
	// "errors"
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// Provider - Exported function that creates the JupiterOne Terraform
// resource provider
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("JUPITERONE_API_KEY", nil),
				Description: "API Key used to make requests to the JupiterOne APIs",
			},
			"account_id": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("JUPITERONE_ACCOUNT_ID", nil),
				Description: "JupiterOne account ID to create resources in",
			},
			"region": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("JUPITERONE_REGION", nil),
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"jupiterone_rule":     resourceQuestionRuleInstance(),
			"jupiterone_question": resourceQuestion(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

// ProviderConfiguration contains the initialized API client to communicate with the JupiterOne API
type ProviderConfiguration struct {
	Client *client.JupiterOneClient
}

func providerConfigure(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	log.Println("[INFO] JupiterOne client successfully initialized")

	config := client.JupiterOneClientConfig{
		APIKey:    d.Get("api_key").(string),
		AccountID: d.Get("account_id").(string),
		Region:    d.Get("region").(string),
	}

	client, err := config.Client()

	if err != nil {
		return nil, diag.Errorf("failed to create JupiterOne client in provider configuration: %s", err.Error())
	}

	return &ProviderConfiguration{
		Client: client,
	}, nil
}
