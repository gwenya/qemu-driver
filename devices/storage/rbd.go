package storage

import (
	"fmt"
	"strings"

	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/qmp"
)

type rbdDrive struct {
	id   DiskIdentifier
	pool string
	name string
}

func NewRbdDrive(id DiskIdentifier, pool string, name string) RbdDrive {
	// TODO: take vendor/product/serial triplet instead of just serial
	return &rbdDrive{
		id:   id,
		pool: pool,
		name: name,
	}
}

func (d *rbdDrive) Config() []config.Section {
	return nil
}

func (d *rbdDrive) GetScsiHotplug(bus string) devices.HotplugDevice {
	return wrapScsiHotplug(d, bus)
}

func (d *rbdDrive) Plug(m qmp.Monitor, bus string) error {
	nodeName := d.id.NodeName()

	blocks, err := m.QueryNamedBlockNodes()
	if err != nil {
		return err
	}

	blockdevExists := false

	for _, block := range blocks {
		if block.NodeName != nodeName {
			continue
		}

		blockdevExists = true
		break
	}

	if !blockdevExists {
		err := m.AddBlockDevice(map[string]any{
			"cache": map[string]any{
				"direct":   false,
				"no-flush": false,
			},
			"discard":   "unmap",
			"driver":    "rbd",
			"pool":      d.pool,
			"image":     d.name,
			"node-name": d.id.NodeName(),
			"read-only": false,
		})

		if err != nil {
			return fmt.Errorf("adding block device: %w", err)
		}
	}

	id := fmt.Sprintf("scsi-%s", d.id.NodeName())

	deviceExists := true

	_, err = m.QomList(fmt.Sprintf("/machine/peripheral/%s", id))
	if err != nil && strings.Contains(err.Error(), "not found") {
		deviceExists = false
	} else if err != nil {
		return err
	}

	if !deviceExists {
		err = m.AddDevice(map[string]any{
			"id":      id,
			"drive":   d.id.NodeName(),
			"vendor":  d.id.Vendor,
			"product": d.id.Product,
			"serial":  d.id.Serial,
			"channel": 0,
			"lun":     1,
			"bus":     bus,
			"driver":  "scsi-hd",
		})

		if err != nil {
			return fmt.Errorf("adding device: %w", err)
		}
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
