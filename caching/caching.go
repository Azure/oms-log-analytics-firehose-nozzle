package caching

import (
	"fmt"
	"os"
	"sync"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

type AppInfo struct {
	Name    string `json:"name"`
	Org     string `json:"org"`
	OrgID   string `json:"orgId"`
	Space   string `json:"space"`
	SpaceID string `json:"spaceId"`
}

type Caching struct {
	cfClientConfig *cfclient.Config
	appInfosByGuid map[string]AppInfo
	appInfoLock    sync.RWMutex
	logger         lager.Logger
	instanceName   string
	environment    string
}

type CachingClient interface {
	GetAppInfo(string) AppInfo
	GetInstanceName() string
	GetEnvironmentName() string
	Initialize()
}

func NewCaching(config *cfclient.Config, logger lager.Logger, environment string) CachingClient {
	return &Caching{
		cfClientConfig: config,
		appInfosByGuid: make(map[string]AppInfo),
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
		var appInfo = AppInfo{
			Name:    app.Name,
			Org:     app.SpaceData.Entity.OrgData.Entity.Name,
			OrgID:   app.SpaceData.Entity.OrgData.Entity.Guid,
			Space:   app.SpaceData.Entity.Name,
			SpaceID: app.SpaceData.Entity.Guid,
		}
		c.appInfosByGuid[app.Guid] = appInfo
		c.logger.Debug("adding to app info cache",
			lager.Data{"guid": app.Guid},
			lager.Data{"info": appInfo})
	}

	c.logger.Info("Cache initialize completed",
		lager.Data{"cache size": len(c.appInfosByGuid)})
}

func (c *Caching) GetAppInfo(appGuid string) AppInfo {
	var appInfo AppInfo
	var ok bool
	func() {
		c.appInfoLock.RLock()
		defer c.appInfoLock.RUnlock()
		appInfo, ok = c.appInfosByGuid[appGuid]
	}()
	if ok {
		return appInfo
	} else {
		c.logger.Info("App info not found for GUID",
			lager.Data{"guid": appGuid})
		// call the client api to get the name for this app
		// purposely create a new client due to issue in using a single client
		cfClient, err := cfclient.NewClient(c.cfClientConfig)
		if err != nil {
			c.logger.Error("error creating cfclient", err)
			return AppInfo{
				Name:    "",
				Org:     "",
				OrgID:   "",
				Space:   "",
				SpaceID: "",
			}
		}
		app, err := cfClient.AppByGuid(appGuid)
		if err != nil {
			c.logger.Error("error getting app info", err, lager.Data{"guid": appGuid})
			return AppInfo{
				Name:    "",
				Org:     "",
				OrgID:   "",
				Space:   "",
				SpaceID: "",
			}
		} else {
			// store app info in map
			appInfo = AppInfo{
				Name:    app.Name,
				Org:     app.SpaceData.Entity.OrgData.Entity.Name,
				OrgID:   app.SpaceData.Entity.OrgData.Entity.Guid,
				Space:   app.SpaceData.Entity.Name,
				SpaceID: app.SpaceData.Entity.Guid,
			}
			func() {
				c.appInfoLock.Lock()
				defer c.appInfoLock.Unlock()
				c.appInfosByGuid[app.Guid] = appInfo
			}()
			c.logger.Debug("adding to app info cache",
				lager.Data{"guid": app.Guid},
				lager.Data{"info": appInfo})
			// return the app name
			return appInfo
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
