package component

type IoTSensor struct {
	SensorId string  `mapstructure:"sensor_id"`
	Mean     float64 `mapstructure:"mean"`
	Std      float64 `mapstructure:"standard_deviation"`
	Copy     int     `mapstructure:"copy"`
}
