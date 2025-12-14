package driver

type PhysicalNetworkInterface struct {
	NetworkInterfaceBase
	Name string
}

func (PhysicalNetworkInterface) __networkInterfaceMarker() {}
