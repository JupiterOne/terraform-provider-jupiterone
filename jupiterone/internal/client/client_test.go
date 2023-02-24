package client

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGraphqlEndpointSetsDefaultValue(t *testing.T) {
	config := JupiterOneClientConfig{}
	endpoint := config.getGraphQLEndpoint(context.TODO())
	assert.Equal(t, endpoint, "https://graphql.us.jupiterone.io/", "Endpoints should match")
}

func TestGetGraphqlEndpointNoOverride(t *testing.T) {
	config := JupiterOneClientConfig{
		Region: "dev",
	}

	endpoint := config.getGraphQLEndpoint(context.TODO())
	assert.Equal(t, endpoint, "https://graphql.dev.jupiterone.io/", "Endpoints should match")
}
