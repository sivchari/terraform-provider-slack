package internal

import (
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

const (
	providerConfig = `
provider "slack" {
	token = "test"
}`
)

func protoV6ProviderFactories(client APIClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"slack": providerserver.NewProtocol6WithError(
			&SlackProvider{
				client: client,
			},
		),
	}
}
