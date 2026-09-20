package pcie

import (
	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/qmp"
)

type physicalNetworkDevice struct {
	hotPlug
	id string
}

func NewPhysicalNetworkDevice(id string, netdevName string) BusDevice {
	return &physicalNetworkDevice{
		id: id,
	}
}

func (d *physicalNetworkDevice) Config(_ BusAllocation) []config.Section {
	return nil
}

func (d *physicalNetworkDevice) GetHotplugs(alloc BusAllocation) []devices.HotplugDevice {
	return []devices.HotplugDevice{
		hotplugWrap(d, alloc),
	}
}

func (d *physicalNetworkDevice) Plug(m qmp.Monitor, alloc BusAllocation) error {
	// TODO implement me
	panic("implement me")
}

func (d *physicalNetworkDevice) Unplug(m qmp.Monitor, alloc BusAllocation) error {
	// TODO implement me
	panic("implement me")
}
