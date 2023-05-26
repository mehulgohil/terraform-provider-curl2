package main

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/mehulgohil/terraform-provider-curl2/curl2"
	"log"
)

// Run the docs generation tool, check its repository for more information on how it works and how docs
// can be customized.
//
//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

func main() {
	err := providerserver.Serve(context.Background(), curl2.NewProvider, providerserver.ServeOpts{
		Address: "example.io/example/curl2",
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
