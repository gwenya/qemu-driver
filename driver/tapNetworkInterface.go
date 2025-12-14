package driver

type TapNetworkInterface struct {
	NetworkInterfaceBase
	Name string
}

func (TapNetworkInterface) __networkInterfaceMarker() {}
