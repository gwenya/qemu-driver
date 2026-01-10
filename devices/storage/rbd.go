package storage

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/qmp"
)

type rbdDrive struct {
	serial string
	pool   string
	name   string
}

func NewRbdDrive(serial string, pool string, name string) RbdDrive {
	// TODO: take vendor/product/serial triplet instead of just serial
	return &rbdDrive{
		serial: serial,
		pool:   pool,
		name:   name,
	}
}

func (d *rbdDrive) Config() []config.Section {
	return nil
}

func (d *rbdDrive) GetScsiHotplug(bus string) devices.HotplugDevice {
	return wrapScsiHotplug(d, bus)
}

func (d *rbdDrive) Plug(m qmp.Monitor, bus string) error {
	err := m.AddBlockDevice(map[string]any{
		"cache": map[string]any{
			"direct":   false,
			"no-flush": false,
		},
		"discard":   "unmap", // Forward as an unmap request. This is the same as `discard=on` in the qemu config file.
		"driver":    "rbd",
		"pool":      d.pool,
		"image":     d.name,
		"node-name": d.serial,
		"read-only": false,
	})

	if err != nil {
		return fmt.Errorf("adding block device: %w", err)
	}

	err = m.AddDevice(map[string]any{
		"id":      fmt.Sprintf("scsi-%s", d.serial),
		"drive":   d.serial,
		"serial":  d.serial,
		"channel": 0,
		"lun":     1,
		"bus":     bus,
		"driver":  "scsi-hd",
	})

	if err != nil {
		return fmt.Errorf("adding device: %w", err)
	}

	return nil
}

func (d *rbdDrive) Unplug(m qmp.Monitor, bus string) error {
	//TODO implement me
	panic("implement me")
}

type RbdDrive interface {
	ScsiDrive
	BlkDrive
}
