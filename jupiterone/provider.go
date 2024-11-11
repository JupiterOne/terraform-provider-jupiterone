package jupiterone

import (
	// "errors"
	"context"
	"log"
	"os"

	"github.com/Khan/genqlient/graphql"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone/internal/client"
)

// JupiterOneProvider contains the initialized API client to communicate with the JupiterOne API
type JupiterOneProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
	Qlient  graphql.Client
}

type JupiterOneProviderModel struct {
	APIKey    basetypes.StringValue `tfsdk:"api_key"`
	AccountID basetypes.StringValue `tfsdk:"account_id"`
	Region    basetypes.StringValue `tfsdk:"region"`
}

var _ provider.Provider = &JupiterOneProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &JupiterOneProvider{
			version: version,
		}
	}
}

// Configure implements provider.Provider
func (p *JupiterOneProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data JupiterOneProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// NOTE: One important use case here is client already being set at part
	// of the acceptance tests to use the preconfigured `go-vcr` transport.
	if p.Qlient == nil {
		apiKey := data.APIKey.ValueString()
		accountId := data.AccountID.ValueString()
		region := data.Region.ValueString()

		// Check environment variables. Performing this as part of Configure is
		// the current de-facto way of "merging" defaults:
		// https://github.com/hashicorp/terraform-plugin-framework/issues/539#issuecomment-1334470425
		if apiKey == "" {
			apiKey = os.Getenv("JUPITERONE_API_KEY")
		}
		if accountId == "" {
			accountId = os.Getenv("JUPITERONE_ACCOUNT_ID")
		}
		if region == "" {
			region = os.Getenv("JUPITERONE_REGION")
		}

		if apiKey == "" {
			resp.Diagnostics.AddError(
				"Missing API key Configuration",
				"While configuring the provider, the API key was not found in "+
					"the JUPITERONE_API_KEY environment variable or provider "+
					"configuration block api_key attribute.",
			)
			// Not returning early allows the logic to collect all errors.
		}

		if accountId == "" {
			resp.Diagnostics.AddError(
				"Missing Account ID Configuration",
				"While configuring the provider, the account id was not found in "+
					"the JUPITERONE_ACCOUNT_ID variable or provider "+
					"configuration block account_id attribute.",
			)
			// Not returning early allows the logic to collect all errors.
		}

		if region == "" {
			resp.Diagnostics.AddError(
				"Missing region Configuration",
				"While configuring the provider, the region was not found in "+
					"the JUPITERONE_REGION variable or provider "+
					"configuration block region attribute.",
			)
			// Not returning early allows the logic to collect all errors.
		}

		config := client.JupiterOneClientConfig{
			APIKey:    apiKey,
			AccountID: accountId,
			Region:    region,
		}

		p.Qlient = config.Qlient(ctx)
		log.Println("[INFO] JupiterOne client successfully initialized")
	} else {
		log.Println("[INFO] Using already configured client")
	}

	resp.DataSourceData = p
	resp.ResourceData = p
}

// DataSources implements provider.Provider
func (*JupiterOneProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewUserGroupDataSource,
	}
}

// Metadata implements provider.Provider
func (p *JupiterOneProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "jupiterone"
	resp.Version = p.version
}

// Resources implements provider.Provider
func (*JupiterOneProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewQuestionResource,
		NewQuestionRuleResource,
		NewFrameworkResource,
		NewGroupResource,
		NewFrameworkItemResource,
		NewLibraryItemResource,
		NewUserGroupResource,
		NewUserGroupMembershipResource,
		NewDashboardResource,
		NewWidgetResource,
		NewDashboardParameterResource,
		NewIntegrationResource,
		NewResourcePermissionResource,
	}
}

// Schema implements provider.Provider
func (*JupiterOneProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				// TODO: needs to be optional to use env vars in Configure
				Optional:    true,
				Description: "API Key used to make requests to the JupiterOne APIs",
				Sensitive:   true,
			},
			"account_id": schema.StringAttribute{
				// TODO: needs to be optional to use env vars in Configure
				Optional:    true,
				Description: "JupiterOne account ID to create resources in",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "region used for generating the GraphQL endpoint url. If not provided defaults to 'us'",
			},
		},
	}
}
