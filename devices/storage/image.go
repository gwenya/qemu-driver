package storage

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/qmp"
)

type imageDriveType string

const (
	typeDisk  imageDriveType = "disk"
	typeCdrom imageDriveType = "cdrom"
)

type imageDrive struct {
	id   string
	path string
	typ  imageDriveType
}

type ImageDrive interface {
	ScsiDrive
	BlkDrive
}

func NewImageDrive(id string, path string) ImageDrive {
	return &imageDrive{
		id:   id,
		path: path,
		typ:  typeDisk,
	}
}

func NewCdromDrive(id string, path string) ImageDrive {
	return &imageDrive{
		id:   id,
		path: path,
		typ:  typeCdrom,
	}
}

func (d *imageDrive) Config() []config.Section {
	return nil
}

func (d *imageDrive) GetScsiHotplug(bus string) devices.HotplugDevice {
	return wrapScsiHotplug(d, bus)
}

func (d *imageDrive) Plug(m qmp.Monitor, bus string) error {
	nodeName := "node-" + d.id
	var driver string
	var readonly bool

	switch d.typ {
	case typeDisk:
		driver = "scsi-hd"
	case typeCdrom:
		driver = "scsi-cd"
		readonly = true
	}

	err := m.AddBlockDevice(map[string]any{
		"aio": "io_uring",
		"cache": map[string]any{
			"direct":   true,
			"no-flush": false,
		},
		"discard":   "unmap", // Forward as an unmap request. This is the same as `discard=on` in the qemu config file.
		"driver":    "file",
		"node-name": nodeName,
		"read-only": readonly,
		"locking":   "off",
		"filename":  d.path,
	})

	if err != nil {
		return fmt.Errorf("adding block device: %w", err)
	}

	err = m.AddDevice(map[string]any{
		"id":        d.id,
		"drive":     nodeName,
		"serial":    d.id,
		"device_id": nodeName,
		"channel":   0,
		"lun":       1,
		"bus":       bus,
		"driver":    driver,
	})

	if err != nil {
		return fmt.Errorf("adding device: %w", err)
	}

	return nil
}

func (d *imageDrive) Unplug(m qmp.Monitor, bus string) error {
	panic("implement me")
}
