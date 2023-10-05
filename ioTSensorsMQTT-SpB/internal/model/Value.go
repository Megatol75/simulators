package model

import sparkplug "github.com/Megatol75/simulators/iotSensorsMQTT-SpB/third_party/sparkplug_b"

type Value struct {
	Type  sparkplug.DataType `json:"type,omitempty"`
	Value any                `json:"value,omitempty"`
}
