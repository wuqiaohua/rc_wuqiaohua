package config

import "slices"

type AppCredential struct {
	AppID          string
	AppSecret      string
	AllowedVendors []string
	Enabled        bool
}

var appCredentials = map[string]AppCredential{
	"biz-payment": {
		AppID:          "biz-payment",
		AppSecret:      "payment-secret-for-local-dev",
		AllowedVendors: []string{"crm"},
		Enabled:        true,
	},
}

func DefaultAppCredentials() map[string]AppCredential {
	credentials := make(map[string]AppCredential, len(appCredentials))
	for appID, credential := range appCredentials {
		credential.AllowedVendors = slices.Clone(credential.AllowedVendors)
		credentials[appID] = credential
	}
	return credentials
}
