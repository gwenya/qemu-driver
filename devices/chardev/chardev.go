package chardev

import (
	"fmt"
	"maps"

	"github.com/gwenya/qemu-driver/config"
)

type Chardev interface {
	Config() config.Section
}

func chardevConfig(id string, backend string, options map[string]string) config.Section {
	entries := make(map[string]string)
	if options != nil {
		maps.Copy(entries, options)
	}

	entries["backend"] = backend

	return config.Section{
		Name:    fmt.Sprintf(`chardev "%s"`, id),
		Entries: entries,
	}
}
