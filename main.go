package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/sivchari/terraform-provider-slack/internal"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.20.1

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Debug:   debugMode,
		Address: "registry.terraform.io/sivchari/slack",
	}

	if err := providerserver.Serve(context.Background(), internal.New(), opts); err != nil {
		log.Fatal(err)
	}
}
