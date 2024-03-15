package simulators

import (
	"math"
	"math/rand"
	"time"
)

type IoTSensorSim struct {
	// Sensor Id
	SensorId string
	// sensor data mean value
	mean float64
	// sensor data standard deviation value
	standardDeviation float64
	// sensor data current value
	currentValue float64

	// Shutdown the sensor
	Shutdown chan bool

	// Check if it's running
	IsRunning bool

	// Sensor Alias, to be used in DDATA, instead of name
	Alias uint64

	// Check if it's already assigned to a device,
	// it's only allowed to be be assigned to one device
	IsAssigned *bool

	// Used to Update sensor parameters at runtime
	Update chan UpdateSensorParams
}

type UpdateSensorParams struct {
	// Sensor Id
	SensorId string
	// sensor data mean value
	Mean float64
	// sensor data standard deviation value
	Std float64
	// Delay between each data point
	DelayMin uint32
	DelayMax uint32
	// Randomize delay between data points if true,
	// otherwise DelayMin will be set as fixed delay
	Randomize bool
}

type SensorData struct {
	Value     float64
	Timestamp time.Time
}

func NewIoTSensorSim(
	id string,
	mean,
	standardDeviation float64,
) *IoTSensorSim {
	isAssigned := false

	return &IoTSensorSim{
		SensorId:          id,
		mean:              mean,
		standardDeviation: math.Abs(standardDeviation),
		currentValue:      mean - rand.Float64(),
		IsRunning:         false,
		IsAssigned:        &isAssigned,
		// Add a buffered channel with capacity 1
		// to send a shutdown signal from the device.
		Shutdown: make(chan bool, 1),
		Update:   make(chan UpdateSensorParams, 1),
	}
}

func (s *IoTSensorSim) CalculateNextValue() SensorData {
	// first calculate how much the value will be changed
	valueChange := rand.Float64() * math.Abs(s.standardDeviation) / 10
	// second decide if the value is increased or decreased
	factor := s.decideFactor()
	// apply valueChange and factor to value and return
	s.currentValue += valueChange * factor
	timestamp := time.Now().Local()
	return SensorData{
		Value:     s.currentValue,
		Timestamp: timestamp,
	}
}

func (s *IoTSensorSim) decideFactor() float64 {
	var (
		continueDirection, changeDirection float64
		distance                           float64 // the distance from the mean.
	)
	// depending on if the current value is smaller or bigger than the mean
	// the direction changes.
	if s.currentValue > s.mean {
		distance = s.currentValue - s.mean
		continueDirection = 1
		changeDirection = -1
	} else {
		distance = s.mean - s.currentValue
		continueDirection = -1
		changeDirection = 1
	}
	// the chance is calculated by taking half of the standardDeviation
	// and subtracting the distance divided by 50. This is done because
	// chance with a distance of zero would mean a 50/50 chance for the
	// randomValue to be higher or lower.
	// The division by 50 was found by empiric testing different values.
	chance := (s.standardDeviation / 2) - (distance / 50)
	randomValue := s.standardDeviation * rand.Float64()
	// if the random value is smaller than the chance we continue in the
	// current direction if not we change the direction.
	if randomValue < chance {
		return continueDirection
	}
	return changeDirection
}
