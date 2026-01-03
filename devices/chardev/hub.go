package chardev

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
)

type hubChardev struct {
	id       string
	backends []string
}

func NewHub(id string, backends ...string) Chardev {
	return &hubChardev{
		id:       id,
		backends: backends,
	}
}

func (c *hubChardev) Config() config.Section {
	opts := make(map[string]string, len(c.backends))

	for i, backend := range c.backends {
		opts[fmt.Sprintf("chardevs.%d", i)] = backend
	}

	return chardevConfig(c.id, "hub", opts)
}
