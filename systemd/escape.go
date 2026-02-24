package systemd

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

// from coreos/go-systemd/dbus/dbus.go

const (
	alpha    = `abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`
	num      = `0123456789`
	alphanum = alpha + num
)

func needsEscape(i int, b byte) bool {
	// Escape everything that is not a-z-A-Z-0-9
	// Also escape 0-9 if it's the first character
	return strings.IndexByte(alphanum, b) == -1 ||
		(i == 0 && strings.IndexByte(num, b) != -1)
}

func PathBusEscape(path string) dbus.ObjectPath {
	// Special case the empty string
	if len(path) == 0 {
		return "_"
	}
	n := []byte{}
	for i := 0; i < len(path); i++ {
		c := path[i]
		if needsEscape(i, c) {
			e := fmt.Sprintf("_%x", c)
			n = append(n, []byte(e)...)
		} else {
			n = append(n, c)
		}
	}
	return dbus.ObjectPath(n)
}

func PathBusUnescape(path string) string {
	if path == "_" {
		return ""
	}
	n := []byte{}
	for i := 0; i < len(path); i++ {
		c := path[i]
		if c == '_' && i+2 < len(path) {
			res, err := hex.DecodeString(path[i+1 : i+3])
			if err == nil {
				n = append(n, res...)
			}
			i += 2
		} else {
			n = append(n, c)
		}
	}
	return string(n)
}
