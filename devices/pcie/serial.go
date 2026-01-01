package pcie

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/devices/serial"
)

type serialBus struct {
	devices.BusImpl[serial.BusDevice]
	noHotPlug
	id  string
	rng string
}

type SerialBus interface {
	serial.Bus
	BusDevice
}

func NewSerialBus(id string) SerialBus {
	return &serialBus{
		id: id,
	}
}

func (d *serialBus) Config(alloc BusAllocation) []config.Section {
	sections := []config.Section{
		busDeviceConfigSection(alloc, d.id, "virtio-serial-pci", nil),
	}

	for _, device := range d.Devices {
		sections = append(sections, device.Config(fmt.Sprintf("%s.0", d.id))...)
	}

	return sections
}
