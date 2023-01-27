package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

var (
	// these will be set by the goreleaser configuration
	// to appropriate values for the compiled binary
	version string = "dev"

	// goreleaser can also pass the specific commit if you want
	// commit  string = ""
)

func main() {
	err := providerserver.Serve(
		context.Background(),
		jupiterone.New(version),
		providerserver.ServeOpts{
			Address: "registry.terraform.io/jupiterone/jupiterone",
		},
	)

	if err != nil {
		log.Fatal(err)
	}
}
