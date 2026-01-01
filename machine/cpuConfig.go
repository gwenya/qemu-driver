package machine

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
)

type cpuConfig struct {
	sockets int
	cores   int
	threads int
}

func (c *cpuConfig) Config() config.Section {
	return config.Section{
		Name: "smp-opts",
		Entries: map[string]string{
			"sockets": fmt.Sprintf("%d", c.sockets),
			"cores":   fmt.Sprintf("%d", c.cores),
			"threads": fmt.Sprintf("%d", c.threads),
		},
	}
}
