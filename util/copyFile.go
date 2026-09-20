package util

import (
	"io"
	"os"
)

func CopyFile(src string, dst string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = srcFile.Close() }()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	defer func() {
		cerr := dstFile.Close()
		if cerr != nil && err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}
