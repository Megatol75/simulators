package services

import (
	"context"
	"strings"
	"time"

	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/component"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/model"
	sparkplug "github.com/Megatol75/simulators/iotSensorsMQTT-SpB/third_party/sparkplug_b"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
	"github.com/matishsiao/goInfo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	proto "google.golang.org/protobuf/proto"
)

var (
	// EoD Node Seq and BdSeq
	Seq        uint64 = 0
	BdSeq      uint64 = 0
	Alias      uint64 = 1
	StartTime  time.Time
	AppVersion string = "v1.0.0"
	Maintainer string = "Amine Amaach"
	Website    string = "amineamaach.me"
	SourceCode string = "https://github.com/amineamaach/simulators"
)

var (
	AckMsgs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "IoTSensors_project",
		Subsystem: "SparkplugB",
		Name:      "acknowledged_messages",
		Help:      "Number of acknowledged messages by the broker",
	})

	UnAckMsgs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "ioTSensors_project",
		Subsystem: "SparkplugB",
		Name:      "unacknowledged_messages",
		Help:      "Number of unacknowledged messages by the broker",
	})

	CachedMsgs = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "ioTSensors_project",
		Subsystem: "SparkplugB",
		Name:      "cached_messages",
		Help:      "Number of cached messages waiting to processed",
	})
)

// EdgeNodeSvc struct describes the EoN Node properties
type EdgeNodeSvc struct {
	Namespace      string
	GroupId        string
	NodeId         string
	Devices        map[string]*DeviceSvc
	SessionHandler *MqttSessionSvc
}

// NewEdgeNodeInstance used to instantiate a new instance of the EoN Node.
func NewEdgeNodeInstance(
	ctx context.Context,
	namespace, groupId, nodeId string,
	bdSeq uint64,
	log *logrus.Logger,
	mqttConfigs *component.MQTTConfig,
) (*EdgeNodeSvc, error) {
	log.Debugln("Setting up a new EoN Node instance 🔔")

	mqttSession := &MqttSessionSvc{
		Log:         log,
		MqttConfigs: *mqttConfigs,
	}

	eonNode := &EdgeNodeSvc{
		Namespace:      namespace,
		GroupId:        groupId,
		NodeId:         nodeId,
		SessionHandler: mqttSession,
		Devices:        make(map[string]*DeviceSvc),
	}

	willTopic := namespace + "/" + groupId + "/NDEATH/" + nodeId

	// Building up the Death Certificate MQTT Payload.
	payload := model.NewSparkplubBPayload(time.Now(), GetNextSeqNum(log)).
		AddMetric(*model.NewMetric("bdSeq", sparkplug.DataType_UInt64, 1, bdSeq))

	// Encoding the Death Certificate MQTT Payload.
	bytes, err := NewSparkplugBEncoder(log).GetBytes(payload)
	if err != nil {
		log.WithFields(logrus.Fields{
			"Groupe ID": eonNode.GroupId,
			"Node ID":   eonNode.NodeId,
		}).Errorln("Error encoding the sparkplug payload ⛔")
		return nil, err
	}

	err = mqttSession.EstablishMqttSession(ctx, willTopic, bytes,
		func(cm *autopaho.ConnectionManager, c *paho.Connack) {
			log.WithFields(logrus.Fields{
				"Groupe Id": eonNode.GroupId,
				"Node Id":   eonNode.NodeId,
			}).Infoln("MQTT connection up ✅")

			// Subscribe to EoN Node control commands
			topic := namespace + "/" + groupId + "/NCMD/" + nodeId

			if _, err := cm.Subscribe(ctx, &paho.Subscribe{
				Subscriptions: map[string]paho.SubscribeOptions{
					topic: {QoS: mqttConfigs.QoS},
				},
			}); err != nil {
				log.Infof("Failed to subscribe (%s). This is likely to mean no messages will be received. ⛔\n", err)
				return
			}
			log.WithField("Topic", topic).Infoln("MQTT subscription made ✅")
		},
		paho.NewSingleHandlerRouter(func(p *paho.Publish) {
			eonNode.OnMessageArrived(ctx, p, log)
		}),
	)

	if err != nil {
		log.WithFields(logrus.Fields{
			"Groupe ID": eonNode.GroupId,
			"Node ID":   eonNode.NodeId,
		}).Errorln("Error establishing MQTT session ⛔")
		return nil, err
	}

	// Register prometheus collectors
	if err := prometheus.Register(AckMsgs); err != nil {
		log.Warnln("Failed to register prometheus collector : [AckMsgs]")
	}
	if err = prometheus.Register(UnAckMsgs); err != nil {
		log.Warnln("Failed to register prometheus collector : [UnAckMsgs]")
	}
	if err := prometheus.Register(CachedMsgs); err != nil {
		log.Warnln("Failed to register prometheus collector : [Cached]")
	}

	StartTime = time.Now()
	return eonNode, err
}

// PublishBirth used to publish the EoN node NBIRTH certificate to the broker.
func (e *EdgeNodeSvc) PublishBirth(ctx context.Context, log *logrus.Logger) *EdgeNodeSvc {
	// The first MQTT message that an EoN node MUST publish upon the successful establishment
	// of an MQTT Session is an EoN BIRTH Certificate.
	props, _ := goInfo.GetInfo()
	upTime := int64(time.Since(StartTime) / 1e+6)
	Seq = 0
	Alias = 1
	alias20 := GetNextAliasRange(20)
	// Create the EoN Node BIRTH payload
	payload := model.NewSparkplubBPayload(time.Now(), GetNextSeqNum(log)).
		AddMetric(*model.NewMetric("bdSeq", sparkplug.DataType_UInt64, alias20, BdSeq)).
		AddMetric(*model.NewMetric("Node Id", sparkplug.DataType_String, alias20+1, e.NodeId)).
		AddMetric(*model.NewMetric("Group Id", sparkplug.DataType_String, alias20+2, e.GroupId)).
		AddMetric(*model.NewMetric("Maintainer", sparkplug.DataType_String, alias20+3, Maintainer)).
		AddMetric(*model.NewMetric("Website", sparkplug.DataType_String, alias20+4, Website)).
		AddMetric(*model.NewMetric("App version", sparkplug.DataType_String, alias20+5, AppVersion)).
		AddMetric(*model.NewMetric("Source code", sparkplug.DataType_String, alias20+6, SourceCode)).
		AddMetric(*model.NewMetric("Up Time ms", sparkplug.DataType_Int64, alias20+7, upTime)).
		AddMetric(*model.NewMetric("Node Control/Rebirth", sparkplug.DataType_Boolean, alias20+8, false)).
		AddMetric(*model.NewMetric("Node Control/Reboot", sparkplug.DataType_Boolean, alias20+9, false)).
		AddMetric(*model.NewMetric("Node Control/Shutdown", sparkplug.DataType_Boolean, alias20+10, false)).
		AddMetric(*model.NewMetric("Node Control/RemoveDevice", sparkplug.DataType_Boolean, alias20+11, false)).
		AddMetric(*model.NewMetric("Node Control/AddDevice", sparkplug.DataType_Boolean, alias20+12, false)).
		AddMetric(*model.NewMetric("Properties/OS", sparkplug.DataType_String, alias20+13, props.OS)).
		AddMetric(*model.NewMetric("Properties/Kernel", sparkplug.DataType_String, alias20+14, props.Kernel)).
		AddMetric(*model.NewMetric("Properties/Core", sparkplug.DataType_String, alias20+15, props.Core)).
		AddMetric(*model.NewMetric("Properties/CPUs", sparkplug.DataType_Int32, alias20+16, int32(props.CPUs))).
		AddMetric(*model.NewMetric("Properties/Platform", sparkplug.DataType_String, alias20+17, props.Platform)).
		AddMetric(*model.NewMetric("Properties/Hostname", sparkplug.DataType_String, alias20+18, props.Hostname))

	for name, d := range e.Devices {
		var i uint64 = 1
		if d != nil {
			upTime := int64(time.Since(d.StartTime) / 1e+6)
			payload.AddMetric(*model.NewMetric("Devices/"+name+"/Up Time ms", sparkplug.DataType_Int64, d.Alias+i, upTime))
		}
	}

	// Encoding the BIRTH Certificate MQTT Payload.
	bytes, err := NewSparkplugBEncoder(log).GetBytes(payload)
	if err != nil {
		log.WithFields(logrus.Fields{
			"Groupe ID": e.GroupId,
			"Node ID":   e.NodeId,
		}).Errorln("Error encoding the EoN Node BIRTH certificate, retrying.. ⛔")
	}

	_, err = e.SessionHandler.MqttClient.Publish(ctx, &paho.Publish{
		Topic:   e.Namespace + "/" + e.GroupId + "/NBIRTH/" + e.NodeId,
		QoS:     1,
		Payload: bytes,
	})

	if err != nil {
		log.WithFields(logrus.Fields{
			"Groupe ID": e.GroupId,
			"Node ID":   e.NodeId,
			"Err":       err,
		}).Errorln("Error publishing the EoN Node BIRTH certificate, retrying.. ⛔")
	} else {
		log.WithFields(logrus.Fields{
			"Groupe Id": e.GroupId,
			"Node Id":   e.NodeId,
		}).Infoln("NBIRTH certificate published successfully ✅")

		// Increment the bdSeq number for the next use
		IncrementBdSeqNum(log)
	}

	return e
}

// OnMessageArrived used to handle the EoN Node incoming control commands
func (e *EdgeNodeSvc) OnMessageArrived(ctx context.Context, msg *paho.Publish, log *logrus.Logger) {
	log.WithField("Topic", msg.Topic).Debugln("New NCMD arrived 🔔")
	isDcmd, deviceId := IsDeviceMessage(msg.Topic)

	if isDcmd {
		log.WithField("DeviceId", deviceId).Infoln("DCMD message")
		deviceToShutdown, exists := e.Devices[deviceId]

		if !exists {
			log.WithField("Device Id", deviceId).Warnln("Device not found 🔔")
			return
		}
		deviceToShutdown.OnMessageArrived(ctx, msg, log)
		return
	}

	var payloadTemplate sparkplug.Payload_Template
	err := proto.Unmarshal(msg.Payload, &payloadTemplate)
	if err != nil {
		log.WithFields(logrus.Fields{
			"Topic": msg.Topic,
			"Err":   err,
		}).Errorln("Failed to unmarshal NCMD payload ⛔")
		return
	}

	for _, metric := range payloadTemplate.Metrics {
		switch *metric.Name {
		case "Node Control/Reboot":
			if value, ok := metric.GetValue().(*sparkplug.Payload_Metric_BooleanValue); !ok {
				log.WithFields(logrus.Fields{
					"Topic": msg.Topic,
					"Value": value,
				}).Errorln("Wrong data type received for this NCMD ⛔")
				return
			} else if value.BooleanValue {
				log.WithField("Node Id", e.NodeId).Infoln("Reboot simulation.. 🔔")
				time.Sleep(time.Duration(5) * time.Second)
				log.WithField("Node Id", e.NodeId).Infoln("Node rebooted successfully ✅")
			}

		case "Node Control/Shutdown":
			if value, ok := metric.GetValue().(*sparkplug.Payload_Metric_BooleanValue); !ok {
				log.WithFields(logrus.Fields{
					"Topic": msg.Topic,
					"Value": value,
				}).Errorln("Wrong data type received for this DCMD ⛔")
			} else if value.BooleanValue {
				//syscall.Kill(syscall.Getpid(), syscall.SIGINT)
			}

		case "Node Control/Rebirth":
			if value, ok := metric.GetValue().(*sparkplug.Payload_Metric_BooleanValue); !ok {
				log.WithFields(logrus.Fields{
					"Topic": msg.Topic,
					"Value": value,
				}).Errorln("Wrong data type received for this NCMD ⛔")
			} else if value.BooleanValue {
				e.PublishBirth(ctx, log)
			}

		case "Node Control/RemoveDevice":
			if value, ok := metric.GetValue().(*sparkplug.Payload_Metric_BooleanValue); !ok {
				log.WithFields(logrus.Fields{
					"Topic": msg.Topic,
					"Value": value,
				}).Errorln("Wrong data type received for this NCMD ⛔")
			} else if value.BooleanValue {
				for _, param := range payloadTemplate.Parameters {
					if *param.Name == "DeviceId" {
						if name, ok := param.Value.(*sparkplug.Payload_Template_Parameter_StringValue); ok {
							e.ShutdownDevice(ctx, name.StringValue, log)
							return
						} else {
							log.WithFields(logrus.Fields{
								"Topic": msg.Topic,
								"Name":  *param.Name,
							}).Errorln("Failed to parse device id ⛔")
							return
						}
					}
				}
				log.WithFields(logrus.Fields{
					"Topic": msg.Topic,
					"Name":  *metric.Name,
				}).Errorln("device id was not found ⛔")
			}

		case "Node Control/AddDevice":
			if value, ok := metric.GetValue().(*sparkplug.Payload_Metric_BooleanValue); !ok {
				log.WithFields(logrus.Fields{
					"Topic": msg.Topic,
					"Value": value,
				}).Errorln("Wrong data type received for this NCMD ⛔")
			} else if value.BooleanValue {
				type AddDevice struct {
					DeviceIdValue string
					TtlValue      uint32
					EnabledValue  bool
				}
				addDevice := AddDevice{}

				for _, param := range payloadTemplate.Parameters {
					if *param.Name == "DeviceId" {
						if name, ok := param.Value.(*sparkplug.Payload_Template_Parameter_StringValue); ok {
							addDevice.DeviceIdValue = name.StringValue
						} else {
							log.WithFields(logrus.Fields{
								"Topic": msg.Topic,
								"Name":  *param.Name,
							}).Errorln("Failed to parse device id ⛔")
							return
						}
					}
					if *param.Name == "StoreAndForward" {
						if enabled, ok := param.Value.(*sparkplug.Payload_Template_Parameter_BooleanValue); ok {
							addDevice.EnabledValue = enabled.BooleanValue
						} else {
							log.WithFields(logrus.Fields{
								"Topic": msg.Topic,
								"Name":  *param.Name,
							}).Errorln("Failed to parse StoreAndForward value ⛔")
							return
						}
					}
					if *param.Name == "TTL" {
						if ttl, ok := param.Value.(*sparkplug.Payload_Template_Parameter_IntValue); ok {
							addDevice.TtlValue = ttl.IntValue
						} else {
							log.WithFields(logrus.Fields{
								"Topic": msg.Topic,
								"Name":  *param.Name,
							}).Errorln("Failed to parse StoreAndForward value ⛔")
							return
						}
					}

				}

				d := NewDeviceInstance(
					ctx,
					e.Namespace,
					e.GroupId,
					e.NodeId,
					addDevice.DeviceIdValue,
					log,
					e.SessionHandler,
					addDevice.TtlValue,
					addDevice.EnabledValue,
				)

				// Add new device
				e.AddDevice(ctx, d, log)
			}

		default:
			log.WithFields(logrus.Fields{
				"Topic": msg.Topic,
				"Name":  *metric.Name,
			}).Errorln("NCMD not defined ⛔")
		}
	}
}

// AddDevice used to add/attach a given device to the EoN Node
func (e *EdgeNodeSvc) AddDevice(ctx context.Context, device *DeviceSvc, log *logrus.Logger) *EdgeNodeSvc {
	if device != nil {
		if device.DeviceId != "" {
			if _, exists := e.Devices[device.DeviceId]; exists {
				log.WithField("Device Id", device.DeviceId).Warnln("Device exists 🔔")
				return e
			}
			e.Devices[device.DeviceId] = device

			log.WithField("Device Id", device.DeviceId).Infoln("Device added successfully ✅")
			//e.PublishBirth(ctx, log)
			return e
		}
		log.Errorln("Device id not set ⛔")
		return e
	}
	log.Errorln("Device is not configured ⛔")
	return e
}

// ShutdownDevice used to shutdown a given device, publish its DDEATH and detached it from the EoN Node.
func (e *EdgeNodeSvc) ShutdownDevice(ctx context.Context, deviceId string, log *logrus.Logger) *EdgeNodeSvc {
	deviceToShutdown, exists := e.Devices[deviceId]
	if !exists {
		log.WithField("Device Id", deviceId).Warnln("Device not found 🔔")
		return e
	}
	deviceToShutdown.connMut.RLock()
	defer deviceToShutdown.connMut.RUnlock()
	delete(e.Devices, deviceId)
	log.WithField("Device Id", deviceId).Debugln("Shutdown all attached sensors.. 🔔")
	for _, sim := range deviceToShutdown.Simulators {
		deviceToShutdown.ShutdownSimulator(ctx, sim.SensorId, log)
	}

	// Building up the Death Certificate MQTT Payload.
	seq := GetNextSeqNum(log)
	payload := model.NewSparkplubBPayload(time.Now(), seq)

	// The Edge of Network (EoN) Node is responsible for publishing DDEATH of its devices.
	// When the EoN Node shuts down unexpectedly, the broker will send its NDEATH as well as
	// all of its attached devices. (Each device sends its DDEATH when initializing connection)

	// Encoding the Death Certificate MQTT Payload.
	bytes, err := NewSparkplugBEncoder(log).GetBytes(payload)
	if err != nil {
		log.WithFields(logrus.Fields{
			"Device ID": deviceId,
			"Err":       err,
			"Info":      "Couldn't create DDEATH certificate",
		}).Errorln("Error encoding the sparkplug payload ⛔")
		return e
	}

	_, err = e.SessionHandler.MqttClient.Publish(ctx, &paho.Publish{
		Topic:   e.Namespace + "/" + e.GroupId + "/DDEATH/" + e.NodeId + "/" + deviceId,
		QoS:     1,
		Payload: bytes,
	})

	if err != nil {
		log.WithFields(logrus.Fields{
			"Device ID": deviceId,
			"Err":       err,
			"Info":      "Device is turned off but couldn't publish DDEATH certificate",
		}).Errorln("Error publishing DDEATH certificate ⛔")
		return e
	}

	deviceToShutdown.SessionHandler.Close(ctx, deviceId)
	deviceToShutdown = nil

	log.WithField("Device Id", deviceId).Infoln("Device removed successfully ✅")
	e.PublishBirth(ctx, log)
	return e
}

func IsDeviceMessage(topic string) (isDcmd bool, deviceId string) {
	topicParts := strings.Split(topic, "/")
	if topicParts[2] == "DCMD" {
		isDcmd = true
		deviceId = topicParts[4]
	} else {
		isDcmd = false
	}

	return isDcmd, deviceId
}

// GetNextSeqNum used to get the sequence number
func GetNextSeqNum(log *logrus.Logger) uint64 {
	retSeq := Seq
	if Seq == 255 {
		Seq = 0
	} else {
		Seq++
	}
	log.WithField("Seq", retSeq).Debugf("Next Seq : %d 🔔\n", Seq)
	return retSeq
}

func GetNextAliasRange(size uint64) uint64 {
	retAlias := Alias
	Alias = Alias + size
	return retAlias
}

// IncrementBdSeqNum used to increment the Bd sequence number
func IncrementBdSeqNum(log *logrus.Logger) {
	if BdSeq == 256 {
		BdSeq = 0
	} else {
		BdSeq++
	}
	log.WithField("Next BdSeq", BdSeq).Debugln("BdSeq incremented 🔔")
}
