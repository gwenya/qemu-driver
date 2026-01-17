package util

import (
	"bytes"
	"fmt"

	"github.com/gwenya/qemu-driver/cmdBuilder"
)

func RunCmd(path string, args ...string) error {
	builder := cmdBuilder.New(path)
	builder.AddArgs(args...)
	var stderr bytes.Buffer
	builder.ConnectStderr(&stderr)
	cmd := builder.Build()
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command %q failed with exit code %w, stderr: %s", cmd, err, stderr.String())
	}

	return nil
}
