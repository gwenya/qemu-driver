package util

import (
	"fmt"
	"os"
)

func RemoveIfExists(path string) error {
	exists, err := FileExists(path)
	if err != nil {
		return fmt.Errorf("checking if %s exists: %w", path, err)
	}

	if exists {
		err = os.Remove(path)
		if err != nil {
			return fmt.Errorf("removing %s: %w", path, err)
		}
	}

	return nil
}
