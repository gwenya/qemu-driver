package machine

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
)

type memoryConfig struct {
	sizeMB    int
	maxSizeMB int
	slots     int
}

func (c *memoryConfig) Config() config.Section {
	section := config.Section{
		Name: "memory",
		Entries: map[string]string{
			"size":   fmt.Sprintf("%dM", c.sizeMB),
			"maxmem": fmt.Sprintf("%dM", c.maxSizeMB),
		},
	}

	if c.slots > 1 {
		section.Entries["slots"] = fmt.Sprintf("%d", c.slots)
	}

	return section
}
