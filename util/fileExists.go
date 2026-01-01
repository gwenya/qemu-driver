package util

import (
	"errors"
	"os"
)

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}
