package component

type EdgeNode struct {
	Namespace string   `mapstructure:"namespace"`
	GroupId   string   `mapstructure:"group_id"`
	NodeId    string   `mapstructure:"node_id"`
	Copy      int      `mapstructure:"copy"`
	Devices   []Device `mapstructure:"devices"`
}
