package pcie

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
)

type vsockDevice struct {
	noHotPlug
	id  string
	cid uint32
	fd  int
}

func NewVsock(id string, cid uint32, fd int) BusDevice {
	return &vsockDevice{
		id:  id,
		cid: cid,
		fd:  fd,
	}
}

func (d *vsockDevice) Config(alloc BusAllocation) []config.Section {
	return []config.Section{
		busDeviceConfigSection(alloc, d.id, "vhost-vsock-pci", map[string]string{
			"guest-cid": fmt.Sprintf("%d", d.cid),
			"vhostfd":   fmt.Sprintf("%d", d.fd),
		}),
	}
}
