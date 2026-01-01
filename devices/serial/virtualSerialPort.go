package serial

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
)

type virtualSerialPort struct {
	id      string
	name    string
	chardev string
}

func NewVirtualSerialPort(id string, name string, chardev string) BusDevice {
	return &virtualSerialPort{
		id:      id,
		name:    name,
		chardev: chardev,
	}
}

func (v *virtualSerialPort) Config(busName string) []config.Section {
	return []config.Section{
		{
			Name: fmt.Sprintf(`device "%s"`, v.id),
			Entries: map[string]string{
				"driver":  "virtserialport",
				"name":    v.name,
				"chardev": v.chardev,
				"bus":     busName,
			},
		},
	}
}
