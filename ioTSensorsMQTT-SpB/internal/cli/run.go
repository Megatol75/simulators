package cli

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/config"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/log"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/services"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/simulators"

	//"github.com/eclipse/paho.golang/paho"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func Run() {

	// Get configs fro file
	cfg := config.GetConfigs()

	// Instantiate a new logger
	logger := log.NewLogger(
		cfg.LoggerConfig.Level,
		cfg.LoggerConfig.Format,
		cfg.LoggerConfig.DisableTimestamp,
	)

	// Instantiate the EoN Node instance
	eodNodeContext := context.Background()
	eonNode, err := services.NewEdgeNodeInstance(
		eodNodeContext,
		cfg.EoNNodeConfig.Namespace,
		cfg.EoNNodeConfig.GroupId,
		cfg.EoNNodeConfig.NodeId,
		services.BdSeq,
		logger,
		&cfg.MQTTConfig,
	)

	if err != nil {
		logger.Errorln("⛔ Failed to instantiate EoN Node, exiting.. ⛔")
		panic(err)
	}

	// Wait for the EoN Node to establish an MQTT connection
	eonNode.SessionHandler.MqttClient.AwaitConnection(eodNodeContext)

	for _, device := range cfg.EoNNodeConfig.Devices {
		deviceContext := context.Background()
		// Instantiate a new device
		newDevice, err := services.NewDeviceInstance(
			deviceContext,
			eonNode.Namespace,
			eonNode.GroupId,
			eonNode.NodeId,
			device.DeviceId,
			logger,
			&cfg.MQTTConfig,
			device.TTL,
			device.StoreAndForward,
		)

		if err != nil {
			logger.WithFields(logrus.Fields{
				"Device Id": device.DeviceId,
				"Err":       err,
			}).Errorln("⛔ Failed to instantiate device ⛔")
			continue
		}

		// // Subscribe to device control commands
		// topic := eonNode.Namespace + "/" + eonNode.GroupId + "/DCMD/" + eonNode.NodeId + "/" + device.DeviceId
		// if _, err := eonNode.SessionHandler.MqttClient.Subscribe(eodNodeContext, &paho.Subscribe{
		// 	Subscriptions: map[string]paho.SubscribeOptions{
		// 		topic: {QoS: cfg.MQTTConfig.QoS},
		// 	},
		// }); err != nil {
		// 	logger.Infof("Failed to subscribe (%s). This is likely to mean no messages will be received. ⛔\n", err)
		// 	return
		// }
		// logger.WithField("Topic", topic).Infoln("MQTT subscription made ✅")

		// Attach the new device to the EoN Node
		eonNode.AddDevice(eodNodeContext, newDevice, logger)

		// Wait for the device to establish an MQTT connection
		eonNode.SessionHandler.MqttClient.AwaitConnection(deviceContext)

		// Add the defined simulated IoTSensors to the new device
		for _, sim := range device.Simulators {
			newDevice.AddSimulator(
				deviceContext,
				simulators.NewIoTSensorSim(
					sim.SensorId,
					sim.Mean,
					sim.Std,
					sim.DelayMin,
					sim.DelayMax,
					sim.Randomize,
				),
				logger,
			).RunSimulators(logger).RunPublisher(deviceContext, logger)
		}
	}

	if cfg.EnablePrometheus {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":8080", nil)
	}

	// Wait for a signal before exiting
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	<-sig

	// We could cancel the context at this point but will call Disconnect instead
	// (this waits for autopaho to shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = eonNode.SessionHandler.MqttClient.Disconnect(ctx)
	for _, d := range eonNode.Devices {
		d.SessionHandler.MqttClient.Disconnect(ctx)
	}

	logger.Info("Shutdown complete ✅")
}
