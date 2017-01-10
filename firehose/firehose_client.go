package firehose

import (
	"crypto/tls"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
	events "github.com/cloudfoundry/sonde-go/events"
)

type Client interface {
	Connect() (<-chan *events.Envelope, <-chan error)
	CloseConsumer() error
}

type client struct {
	cfClientConfig *cfclient.Config
	firehoseConfig *FirehoseConfig
	logger         lager.Logger
	consumer       *consumer.Consumer
}

type FirehoseConfig struct {
	SubscriptionId       string
	TrafficControllerUrl string
	IdleTimeout          time.Duration
}

type CfClientTokenRefresh struct {
	cfClient *cfclient.Client
}

func (ct *CfClientTokenRefresh) RefreshAuthToken() (string, error) {
	return ct.cfClient.GetToken()
}

func NewClient(cfClientConfig *cfclient.Config, firehoseConfig *FirehoseConfig, logger lager.Logger) Client {
	return &client{
		cfClientConfig: cfClientConfig,
		firehoseConfig: firehoseConfig,
		logger:         logger,
	}
}

func (c *client) Connect() (<-chan *events.Envelope, <-chan error) {
	c.logger.Info("connect", lager.Data{"dopplerAddress": c.firehoseConfig.TrafficControllerUrl})
	cfClient, err := cfclient.NewClient(c.cfClientConfig)
	if err != nil {
		c.logger.Fatal("error creating cfclient", err)
	}

	c.consumer = consumer.New(
		c.firehoseConfig.TrafficControllerUrl,
		&tls.Config{InsecureSkipVerify: c.cfClientConfig.SkipSslValidation},
		nil)

	refresher := CfClientTokenRefresh{cfClient: cfClient}
	c.consumer.RefreshTokenFrom(&refresher)
	c.consumer.SetIdleTimeout(c.firehoseConfig.IdleTimeout)
	return c.consumer.Firehose(c.firehoseConfig.SubscriptionId, "")
}

func (c *client) CloseConsumer() error {
	return c.consumer.Close()
}
