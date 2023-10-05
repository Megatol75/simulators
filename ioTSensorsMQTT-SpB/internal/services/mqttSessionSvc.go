package services

import (
	"context"
	"net"
	"net/url"
	"time"

	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/component"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/packets"
	"github.com/eclipse/paho.golang/paho"
	nanoid "github.com/matoous/go-nanoid/v2"
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
	willTopic string,
	payload []byte,
	onConnectionUp func(cm *autopaho.ConnectionManager, c *paho.Connack),
	messageHandler *paho.SingleHandlerRouter,
) error {
	if m.MqttClient != nil {
		m.Log.Warnln("MQTT session already exists 🔔")
		return nil
	}

	m.Log.Debugln("Setting up an MQTT client options 🔔")

	connectTimeout, err := time.ParseDuration(m.MqttConfigs.ConnectTimeout)
	if err != nil {
		m.Log.Errorf("Unable to parse connect timeout duration string: %w ⛔", err)
		return err
	}

	srvURL, err := url.Parse(m.MqttConfigs.URL)
	if err != nil {
		m.Log.Errorf("Unable to parse server URL [%s] : %w ⛔", srvURL, err)
		return err
	}

	var cliId string
	if m.MqttConfigs.ClientID != "" {
		cliId = m.MqttConfigs.ClientID
	} else {
		cliId, err = nanoid.New()
		if err != nil {
			m.Log.Errorln("Unable to auto-generate client id ⛔")
			return err
		}
		cliId = "IoTSensorsMQTT-SpB::" + cliId
	}

	var conn net.Conn
	cliCfg := autopaho.ClientConfig{
		BrokerUrls:        []*url.URL{srvURL},
		KeepAlive:         m.MqttConfigs.KeepAlive,
		ConnectRetryDelay: time.Duration(m.MqttConfigs.ConnectRetry) * time.Second,
		ConnectTimeout:    connectTimeout,
		OnConnectionUp:    onConnectionUp,
		OnConnectError: func(err error) {
			m.Log.Errorf("Error whilst attempting connection %s ⛔\n", err)
		},
		Debug: m.Log,
		// TODO : TlsConfig
		ClientConfig: paho.ClientConfig{
			Router: messageHandler,
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
			Conn: packets.NewThreadSafeConn(conn),
		},
	}
	cliCfg.SetConnectPacketConfigurator(func(c *paho.Connect) *paho.Connect {
		return &paho.Connect{
			ClientID:   cliId,
			KeepAlive:  m.MqttConfigs.KeepAlive,
			CleanStart: m.MqttConfigs.CleanStart,
			Properties: &paho.ConnectProperties{
				SessionExpiryInterval: &m.MqttConfigs.SessionExpiryInterval,
			},
		}
	})
	if m.MqttConfigs.User != "" {
		cliCfg.SetUsernamePassword(m.MqttConfigs.User, []byte(m.MqttConfigs.Password))
	}

	// Setup the Death Certificate Topic/Payload into the MQTT session
	cliCfg.SetWillMessage(willTopic, payload, 0, false)

	// Connect to the broker - this will return immediately after initiating the connection process
	m.Log.Infof("Trying to establish an MQTT Session to %v 🔔\n", cliCfg.BrokerUrls)
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
