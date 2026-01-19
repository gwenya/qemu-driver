package util

import (
	"bytes"
	"fmt"
	"os/exec"
)

func RunCmd(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("command %q failed with exit code %w, stderr: %s", cmd, err, stderr.String())
	}

	return nil
}
