package uaatokenfetcher

import (
	"github.com/cloudfoundry-incubator/uaago"
	"github.com/cloudfoundry/gosteno"
)

type UAATokenFetcher struct {
	uaaUrl                string
	username              string
	password              string
	insecureSSLSkipVerify bool
}

func New(uaaUrl string, username string, password string, sslSkipVerify bool, logger *gosteno.Logger) *UAATokenFetcher {
	return &UAATokenFetcher{
		uaaUrl:                uaaUrl,
		username:              username,
		password:              password,
		insecureSSLSkipVerify: sslSkipVerify,
	}
}

func (uaa *UAATokenFetcher) FetchAuthToken() string {
	uaaClient, err := uaago.NewClient(uaa.uaaUrl)
	if err != nil {
		panic("Error creating uaa client:" + err.Error())
	}

	var authToken string
	authToken, err = uaaClient.GetAuthToken(uaa.username, uaa.password, uaa.insecureSSLSkipVerify)
	if err != nil {
		panic("Error getting oauth token. Please check your username and password. Error:" + err.Error())
	}
	return authToken
}
