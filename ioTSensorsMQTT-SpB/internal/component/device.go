package component

type Device struct {
	DeviceId   string      `mapstructure:"device_id"`
	DelayMin   uint32      `mapstructure:"delay_min"`
	DelayMax   uint32      `mapstructure:"delay_max"`
	Randomize  bool        `mapstructure:"randomize"`
	Simulators []IoTSensor `mapstructure:"simulators"`
}
