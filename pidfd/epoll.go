package pidfd

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type Waiter interface {
	Add(pidfd int) (chan struct{}, error)
	Remove(pidfd int) error

	io.Closer
}

type waiter struct {
	epollFd int
	eventFd int

	fdmap    map[int]chan struct{}
	mu       sync.Mutex
	closeErr chan error
}

func NewWaiter() (Waiter, error) {
	epfd, err := syscall.EpollCreate1(syscall.EPOLL_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("epoll_create1: %w", err)
	}

	efd, err := unix.Eventfd(0, unix.EFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("eventfd: %w", err)
	}

	err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, efd, &syscall.EpollEvent{
		Events: syscall.EPOLLIN,
		Fd:     int32(efd),
	})

	ep := &waiter{
		epollFd:  epfd,
		eventFd:  efd,
		fdmap:    make(map[int]chan struct{}),
		closeErr: make(chan error),
	}

	go ep.run()

	return ep, nil
}

func (e *waiter) Add(pidfd int) (chan struct{}, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.fdmap[pidfd]; exists {
		return nil, fmt.Errorf("already waiting for pidfd %d", pidfd)
	}

	ch := make(chan struct{})
	e.fdmap[pidfd] = ch

	err := syscall.EpollCtl(e.epollFd, syscall.EPOLL_CTL_ADD, pidfd, &syscall.EpollEvent{
		Events: syscall.EPOLLIN | syscall.EPOLLONESHOT,
		Fd:     int32(pidfd),
	})
	if err != nil {
		delete(e.fdmap, pidfd)
		return nil, fmt.Errorf("epoll_ctl: %w", err)
	}

	return ch, nil
}

func (e *waiter) Remove(pidfd int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.fdmap[pidfd]; !exists {
		return fmt.Errorf("pidfd %d is not waited on", pidfd)
	}

	err := syscall.EpollCtl(e.epollFd, syscall.EPOLL_CTL_DEL, pidfd, nil)
	if err != nil {
		return fmt.Errorf("epoll_ctl: %w", err)
	}

	delete(e.fdmap, pidfd)

	return nil
}

func (e *waiter) Close() error {
	value := make([]byte, 8)
	binary.NativeEndian.PutUint64(value, 1)
	_, err := syscall.Write(e.eventFd, value)
	if err != nil {
		return fmt.Errorf("signalling eventfd: %w", err)
	}

	err = <-e.closeErr
	if err != nil {
		return err
	}

	return nil
}

func (e *waiter) run() {
	events := make([]syscall.EpollEvent, 8)

loop:
	for {
		ready, err := syscall.EpollWait(e.epollFd, events, -1)
		if errors.Is(err, syscall.EINTR) || ready == 0 {
			continue
		}
		if err != nil {
			panic(fmt.Errorf("unrecoverable error in epoll_wait: %w", err))
		}

		e.mu.Lock()
		for i := 0; i < ready; i++ {
			event := events[i]

			if event.Fd == int32(e.eventFd) {
				break loop
			}

			// TODO: need to lock the mutex

			ch, ok := e.fdmap[int(event.Fd)]
			if ok {
				close(ch)
				// TODO: remove from the map
				// with EPOLL_CTL_DEL the only errors that can occur are ENOENT (doesn't matter) and perhaps ENOMEM (no way to deal with that)
				_ = syscall.EpollCtl(e.epollFd, syscall.EPOLL_CTL_DEL, int(event.Fd), nil)
			}
		}
		e.mu.Unlock()
	}

	var errs []error

	err := syscall.Close(e.epollFd)
	if err != nil {
		errs = append(errs, fmt.Errorf("closing epoll fd: %w", err))
	}

	err = syscall.Close(e.eventFd)
	if err != nil {
		errs = append(errs, fmt.Errorf("closing event fd: %w", err))
	}

	if len(errs) > 0 {
		e.closeErr <- errors.Join(errs...)
	} else {
		e.closeErr <- nil
	}
}
