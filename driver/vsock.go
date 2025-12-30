package driver

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

/*
#include <linux/vhost.h>
*/
import "C"

func openVsock(cid uint32) (retF *os.File, retErr error) {
	f, err := os.OpenFile("/dev/vhost-vsock", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening /dev/vhost-vsock: %w", err)
	}

	defer func() {
		if retErr != nil {
			_ = f.Close()
		}
	}()

	cidUint64 := uint64(cid)

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), C.VHOST_VSOCK_SET_GUEST_CID, uintptr(unsafe.Pointer(&cidUint64)))
	if errno != 0 {
		if errors.Is(errno, unix.EADDRINUSE) {
			return nil, fmt.Errorf("vsock cid %d is already in use: %w", cid, errno)
		}
		return nil, fmt.Errorf("setting vhost cid: %w", errno)
	}

	return f, nil
}
