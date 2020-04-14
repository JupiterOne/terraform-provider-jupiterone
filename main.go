package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/jupiterone/terraform-provider-jupiterone/jupiterone"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: jupiterone.Provider})
}
