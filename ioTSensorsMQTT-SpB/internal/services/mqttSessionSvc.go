package services

import (
	"context"
	"net/url"
	"time"

	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/component"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/sirupsen/logrus"
)

type MqttSessionSvc struct {
	Log         *logrus.Logger
	MqttConfigs component.MQTTConfig
	MqttClient  *autopaho.ConnectionManager
}

func NewMqttSessionSvc() *MqttSessionSvc {
	return &MqttSessionSvc{}
}

func (m *MqttSessionSvc) EstablishMqttSession(ctx context.Context,
	eonId string,
	willTopic string,
	onConnectionUp func(cm *autopaho.ConnectionManager, c *paho.Connack),
	onConnectionDown func() bool,
	messageHandler func(paho.PublishReceived) (bool, error),
	getNDeathPayload func(log *logrus.Logger) ([]byte, error),
) error {
	if m.MqttClient != nil {
		m.Log.Warnln("MQTT session already exists 🔔")
		return nil
	}

	m.Log.Debugln("Setting up an MQTT client options 🔔")

	connectTimeout, err := time.ParseDuration(m.MqttConfigs.ConnectTimeout)
	if err != nil {
		m.Log.Errorf("Unable to parse connect timeout duration string: %v ⛔", err)
		return err
	}

	srvURL, err := url.Parse(m.MqttConfigs.URL)
	if err != nil {
		m.Log.Errorf("Unable to parse server URL [%s] : %v ⛔", srvURL, err)
		return err
	}

	cliCfg := autopaho.ClientConfig{
		ServerUrls:        []*url.URL{srvURL},
		KeepAlive:         m.MqttConfigs.KeepAlive,
		ConnectRetryDelay: time.Duration(m.MqttConfigs.ConnectRetry) * time.Second,
		ConnectTimeout:    connectTimeout,
		OnConnectionUp:    onConnectionUp,
		OnConnectionDown:  onConnectionDown,
		OnConnectError: func(err error) {
			m.Log.Errorf("Error whilst attempting connection %s ⛔\n", err)
		},
		ConnectPacketBuilder: func(cp *paho.Connect, url *url.URL) (*paho.Connect, error) {
			nDeathPayload, error := getNDeathPayload(m.Log)

			if (error != nil){
				return nil, error
			}

			//setup updated NDEATH after disconnect
			cp.WillMessage = &paho.WillMessage{
				Topic: willTopic,
				Payload: nDeathPayload,
				Retain: false,
				QoS: 1,
			}
			m.Log.Infof("Connecting to host %v", url.Host)
			return cp, nil
		},
		Debug: m.Log,
		// TODO : TlsConfig
		ClientConfig: paho.ClientConfig{
			ClientID: eonId,
			OnPublishReceived: []func(paho.PublishReceived) (bool, error){
				messageHandler,
			},
			OnClientError: func(err error) {
				m.Log.Errorf("Server requested disconnect: %s ⛔\n", err)
			},
			OnServerDisconnect: func(d *paho.Disconnect) {
				if d.Properties != nil {
					m.Log.Errorf("Server requested disconnect: %s ⛔\n", d.Properties.ReasonString)
				} else {
					m.Log.Errorf("Server requested disconnect; reason code : %d ⛔\n", d.ReasonCode)
				}
			},
		},
	}

	if m.MqttConfigs.User != "" {
		cliCfg.ConnectUsername = m.MqttConfigs.User;
		cliCfg.ConnectPassword = []byte(m.MqttConfigs.Password);
	}

	// Connect to the broker - this will return immediately after initiating the connection process
	m.Log.Infof("Trying to establish an MQTT Session to %v for ClientId %s🔔\n", cliCfg.BrokerUrls, eonId)
	cm, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		panic(err)
	}

	m.MqttClient = cm
	return nil
}

func (m *MqttSessionSvc) Close(ctx context.Context, id string) {
	m.Log.WithField("ClientId", id).Debugln("Closing MQTT connection.. 🔔")
	if m.MqttClient != nil {
		if err := m.MqttClient.Disconnect(ctx); err == nil {
			m.Log.WithField("ClientId", id).Infoln("MQTT connection closed ✅")
		}
	}
}
