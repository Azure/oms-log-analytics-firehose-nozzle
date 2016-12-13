package caching

import (
	"fmt"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type Caching struct {
	cfClientConfig *cfclient.Config
	appNamesByGuid map[string]string
}

func NewCaching(config *cfclient.Config) *Caching {
	return &Caching{
		cfClientConfig: config,
		appNamesByGuid: make(map[string]string),
	}
}

func (c *Caching) Initialize() {
	cfClient, err := cfclient.NewClient(c.cfClientConfig)
	if err != nil {
		panic("Error creating cfclient:" + err.Error())
	}

	apps, err := cfClient.ListApps()
	if err != nil {
		panic("Error getting app list:" + err.Error())
	}

	for _, app := range apps {
		fmt.Printf("Adding to AppName cache. App guid:%s name:%s\n", app.Guid, app.Name)
		c.appNamesByGuid[app.Guid] = app.Name
	}
	fmt.Printf("Size of appNamesByGuid:%d\n", len(c.appNamesByGuid))
}

func (c *Caching) GetAppName(appGuid string) (string, error) {
	if appName, ok := c.appNamesByGuid[appGuid]; ok {
		return appName, nil
	} else {
		fmt.Printf("Appname not found for GUID:%s Current size of map:%d\n", appGuid, len(c.appNamesByGuid))
		// call the client api to get the name for this app
		cfClient, err := cfclient.NewClient(c.cfClientConfig)
		if err != nil {
			fmt.Printf("Error creating cfclient:%v\n", err)
			return "", err
		}
		app, err := cfClient.AppByGuid(appGuid)
		if err != nil {
			fmt.Printf("Error getting appname for GUID:%s Error was:%v\n", appGuid, err)
			return "", err
		} else {
			// store appname in map
			c.appNamesByGuid[app.Guid] = app.Name
			fmt.Printf("Adding to AppName cache. App guid:%s name:%s. New size of appNamesByGuid:%d\n", app.Guid, app.Name, len(c.appNamesByGuid))
			// return the app name
			return app.Name, nil
		}
	}
}
