package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/config"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/log"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/services"
	"github.com/Megatol75/simulators/iotSensorsMQTT-SpB/internal/simulators"

	"github.com/eclipse/paho.golang/paho"
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

	eonCount := cfg.EoNNodeConfig.Copy
	eonNodes := make([]*services.EdgeNodeSvc, eonCount)

	for i := 0; i < eonCount; i++ {
		eonNode, err := services.NewEdgeNodeInstance(
			eodNodeContext,
			cfg.EoNNodeConfig.Namespace,
			cfg.EoNNodeConfig.GroupId,
			fmt.Sprintf("%v%05d", cfg.EoNNodeConfig.NodeId, i),
			services.BdSeq,
			logger,
			&cfg.MQTTConfig,
		)

		if err != nil {
			logger.Errorln("⛔ Failed to instantiate EoN Node, exiting.. ⛔")
			panic(err)
		}

		eonNodes[i] = eonNode

		// Wait for the EoN Node to establish an MQTT connection
		eonNode.SessionHandler.MqttClient.AwaitConnection(eodNodeContext)

		for _, device := range cfg.EoNNodeConfig.Devices {
			// Instantiate a new device
			newDevice := services.NewDeviceInstance(
				eonNode,
				eonNode.Namespace,
				eonNode.GroupId,
				eonNode.NodeId,
				device.DeviceId,
				logger,
				eonNode.SessionHandler,
				device.DelayMin,
				device.DelayMax,
				device.Randomize,
			)

			// Attach the new device to the EoN Node
			eonNode.AddDevice(newDevice, logger)

			// Subscribe to device control commands
			topic := eonNode.Namespace + "/" + eonNode.GroupId + "/DCMD/" + eonNode.NodeId + "/" + device.DeviceId
			if _, err := eonNode.SessionHandler.MqttClient.Subscribe(eodNodeContext, &paho.Subscribe{
				Subscriptions: map[string]paho.SubscribeOptions{
					topic: {QoS: cfg.MQTTConfig.QoS},
				},
			}); err != nil {
				logger.Infof("Failed to subscribe (%s). This is likely to mean no messages will be received. ⛔\n", err)
				return
			}
			logger.WithField("Topic", topic).Infoln("MQTT subscription made ✅")

			// Add the defined simulated IoTSensors to the new device
			for _, sim := range device.Simulators {
				numberOfSensors := sim.Copy

				for i := 0; i < numberOfSensors; i++ {
					newDevice.AddSimulator(
						simulators.NewIoTSensorSim(
							fmt.Sprintf("%v%03d", sim.SensorId, i),
							sim.Mean+float64(i),
							sim.Std,
						),
						logger)
				}
			}
		}

		eonNode.PublishBirth(eodNodeContext, logger)

		//publish DBIRTHs
		for _, d := range eonNode.Devices {
			d.PublishBirth(eodNodeContext, logger)
		}

	}

	for _, eonNode := range eonNodes {
		//Start publishing
		for _, d := range eonNode.Devices {
			d.Run(logger).RunPublisher(eodNodeContext, logger)
		}
	}

	// Wait for a signal before exiting
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	<-sig

	// We could cancel the context at this point but will call Disconnect instead
	// (this waits for autopaho to shutdown)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for _, eonNode := range eonNodes {
		eonNode.SessionHandler.MqttClient.Disconnect(ctx)
	}

	logger.Info("Shutdown complete ✅")
}
