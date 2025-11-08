package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
	"kc/internal/config"
)

func Login(ctx context.Context) (*gocloak.GoCloak, string, error) {
	client := gocloak.NewClient(config.Global.ServerURL)
	switch config.Global.GrantType {
	case "client_credentials":
		token, err := client.LoginClient(ctx, config.Global.ClientID, config.Global.ClientSecret, config.Global.AuthRealm)
		if err != nil {
			return nil, "", err
		}
		return client, token.AccessToken, nil
	case "password":
		// Use admin login with username/password for admin operations
		token, err := client.LoginAdmin(ctx, config.Global.Username, config.Global.Password, config.Global.AuthRealm)
		if err != nil {
			return nil, "", err
		}
		return client, token.AccessToken, nil
	default:
		token, err := client.LoginClient(ctx, config.Global.ClientID, config.Global.ClientSecret, config.Global.AuthRealm)
		if err != nil {
			return nil, "", err
		}
		return client, token.AccessToken, nil
	}
}
