package driver

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// VHOST_VSOCK_SET_GUEST_CID is from linux/vhost.h
//
//goland:noinspection GoSnakeCaseUsage
const VHOST_VSOCK_SET_GUEST_CID = 0x4008AF60

const AnyVsockCid uint32 = 0xffffffff

const (
	firstVsockCid uint32 = 3
	lastVsockCid  uint32 = AnyVsockCid - 1
)

const freeVsockTries = 10

func openVsock(cid uint32) (retF *os.File, retErr error) {
	if cid < firstVsockCid || cid == AnyVsockCid {
		return nil, fmt.Errorf("vsock cid %d is reserved", cid)
	}

	f, err := openVhostVsock()
	if err != nil {
		return nil, err
	}

	defer func() {
		if retErr != nil {
			_ = f.Close()
		}
	}()

	err = setGuestCid(f, cid)
	if err != nil {
		if errors.Is(err, unix.EADDRINUSE) {
			return nil, fmt.Errorf("vsock cid %d is already in use: %w", cid, err)
		}
		return nil, fmt.Errorf("setting vhost cid: %w", err)
	}

	return f, nil
}

func openFreeVsock() (retF *os.File, retCid uint32, retErr error) {
	f, err := openVhostVsock()
	if err != nil {
		return nil, 0, err
	}

	defer func() {
		if retErr != nil {
			_ = f.Close()
		}
	}()

	for range freeVsockTries {
		cid := firstVsockCid + rand.Uint32N(lastVsockCid-firstVsockCid+1)

		err = setGuestCid(f, cid)
		if err == nil {
			return f, cid, nil
		}

		if !errors.Is(err, unix.EADDRINUSE) {
			return nil, 0, fmt.Errorf("setting vhost cid: %w", err)
		}
	}

	return nil, 0, fmt.Errorf("no free vsock cid after %d tries", freeVsockTries)
}

func openVhostVsock() (*os.File, error) {
	f, err := os.OpenFile("/dev/vhost-vsock", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening /dev/vhost-vsock: %w", err)
	}

	return f, nil
}

func setGuestCid(f *os.File, cid uint32) error {
	cidUint64 := uint64(cid)

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, f.Fd(), VHOST_VSOCK_SET_GUEST_CID, uintptr(unsafe.Pointer(&cidUint64)))
	if errno != 0 {
		return errno
	}

	return nil
}
