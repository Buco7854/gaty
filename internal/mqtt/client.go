package mqtt

import (
	"fmt"
	"log/slog"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client wraps the paho MQTT client with auto-reconnect.
type Client struct {
	c pahomqtt.Client
}

// New creates a new MQTT client. username and password are optional (empty = anonymous).
func New(brokerURL, username, password string) (*Client, error) {
	opts := pahomqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID("gaty-server").
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(func(_ pahomqtt.Client) {
			slog.Info("mqtt connected")
		}).
		SetConnectionLostHandler(func(_ pahomqtt.Client, err error) {
			slog.Warn("mqtt connection lost", "error", err)
		})

	if username != "" {
		opts.SetUsername(username).SetPassword(password)
	}

	c := pahomqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", token.Error())
	}
	return &Client{c: c}, nil
}

func (c *Client) Publish(topic string, payload []byte) error {
	token := c.c.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

func (c *Client) Subscribe(topic string, handler pahomqtt.MessageHandler) error {
	token := c.c.Subscribe(topic, 1, handler)
	token.Wait()
	return token.Error()
}

func (c *Client) Disconnect() {
	c.c.Disconnect(250)
}