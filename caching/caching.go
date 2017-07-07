package caching

import (
	"code.cloudfoundry.org/lager"
	"fmt"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"os"
)

type Caching struct {
	cfClientConfig *cfclient.Config
	appNamesByGuid map[string]string
	logger         lager.Logger
	instanceName   string
	environment    string
}

type CachingClient interface {
	GetAppName(string) string
	GetInstanceName() string
	GetEnvironmentName() string
	Initialize()
}

func NewCaching(config *cfclient.Config, logger lager.Logger, environment string) CachingClient {
	return &Caching{
		cfClientConfig: config,
		appNamesByGuid: make(map[string]string),
		logger:         logger,
		environment:    environment,
	}
}

func (c *Caching) Initialize() {
	c.setInstanceName()

	cfClient, err := cfclient.NewClient(c.cfClientConfig)
	if err != nil {
		c.logger.Fatal("error creating cfclient", err)
	}

	apps, err := cfClient.ListApps()
	if err != nil {
		c.logger.Fatal("error getting app list", err)
	}

	for _, app := range apps {
		c.appNamesByGuid[app.Guid] = app.Name
		c.logger.Info("adding to app name cache",
			lager.Data{"guid": app.Guid},
			lager.Data{"name": app.Name},
			lager.Data{"cache size": len(c.appNamesByGuid)})
	}
}

func (c *Caching) GetAppName(appGuid string) string {
	if appName, ok := c.appNamesByGuid[appGuid]; ok {
		return appName
	} else {
		c.logger.Info("App name not found for GUID",
			lager.Data{"guid": appGuid},
			lager.Data{"app name cache size": len(c.appNamesByGuid)})
		// call the client api to get the name for this app
		// purposely create a new client due to issue in using a single client
		cfClient, err := cfclient.NewClient(c.cfClientConfig)
		if err != nil {
			c.logger.Error("error creating cfclient", err)
			return ""
		}
		app, err := cfClient.AppByGuid(appGuid)
		if err != nil {
			c.logger.Error("error getting appname", err, lager.Data{"guid": appGuid})
			return ""
		} else {
			// store appname in map
			c.appNamesByGuid[app.Guid] = app.Name
			c.logger.Info("adding to app name cache",
				lager.Data{"guid": app.Guid},
				lager.Data{"name": app.Name},
				lager.Data{"cache size": len(c.appNamesByGuid)})
			// return the app name
			return app.Name
		}
	}
}

func (c *Caching) setInstanceName() error {
	// instance id to track multiple nozzles, used for logging
	hostName, err := os.Hostname()
	if err != nil {
		c.logger.Error("failed to get hostname for nozzle instance", err)
		c.instanceName = fmt.Sprintf("pid-%d", os.Getpid())
	} else {
		c.instanceName = fmt.Sprintf("pid-%d@%s", os.Getpid(), hostName)
	}
	c.logger.Info("getting nozzle instance name", lager.Data{"name": c.instanceName})
	return err
}

func (c *Caching) GetInstanceName() string {
	return c.instanceName
}

func (c *Caching) GetEnvironmentName() string {
	return c.environment
}
