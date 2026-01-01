package chardev

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
)

type ringbufChardev struct {
	id   string
	size int
}

func NewRingbuf(id string, size int) Chardev {
	return &ringbufChardev{
		id:   id,
		size: size,
	}
}

func (c *ringbufChardev) Config() config.Section {
	return chardevConfig(c.id, "ringbuf", map[string]string{
		"size": fmt.Sprintf("%d", c.size),
	})
}
