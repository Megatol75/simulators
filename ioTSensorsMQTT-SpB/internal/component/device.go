package component

type Device struct {
	DeviceId        string      `mapstructure:"device_id"`
	StoreAndForward bool        `mapstructure:"store_and_forward"`
	TTL             uint32      `mapstructure:"time_to_live"`
	DelayMin        uint32      `mapstructure:"delay_min"`
	DelayMax        uint32      `mapstructure:"delay_max"`
	Randomize       bool        `mapstructure:"randomize"`
	Simulators      []IoTSensor `mapstructure:"simulators"`
}
