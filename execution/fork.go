package execution

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/gwenya/qemu-driver/pidfd"
	"golang.org/x/sys/unix"
)

type forkStrategy struct {
	pidFile          string
	pidfdWaiter      pidfd.Waiter
	pidfdWaiterOwned bool
	stdoutFile       string
	stderrFile       string
	quitCh           chan struct{}
	pidFd            int
	logger           Logger
	mu               sync.Mutex
}

type ForkOptions struct {
	PidFilePath    string
	StdoutFilePath string
	StderrFilePath string
	Logger         Logger
	PidFdWaiter    pidfd.Waiter
}

func NewForkStrategy(opts ForkOptions) (Strategy, error) {
	pidfdWaiterOwned := false
	pidfdWaiter := opts.PidFdWaiter
	if pidfdWaiter == nil {
		var err error
		pidfdWaiter, err = pidfd.NewWaiter()
		if err != nil {
			return nil, err
		}
		pidfdWaiterOwned = true
	}

	return &forkStrategy{
		pidFile:          opts.PidFilePath,
		stdoutFile:       opts.StdoutFilePath,
		stderrFile:       opts.StderrFilePath,
		pidfdWaiter:      pidfdWaiter,
		pidfdWaiterOwned: pidfdWaiterOwned,
		logger:           opts.Logger,
		pidFd:            -1,
	}, nil
}

func (s *forkStrategy) FindRunning() (chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pidFileBytes, err := os.ReadFile(s.pidFile)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("opening pid file: %w", err)
	}

	pidInt64, err := strconv.ParseInt(strings.TrimSpace(string(pidFileBytes)), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pid file content %q: %w", string(pidFileBytes), err)
	}

	pid := int(pidInt64)
	fd, err := unix.PidfdOpen(pid, unix.PIDFD_NONBLOCK)
	if errors.Is(err, unix.ESRCH) {
		err := os.Remove(s.pidFile)
		if err != nil {
			return nil, fmt.Errorf("failed to remove old pid file %q: %w", s.pidFile, err)
		}

		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to open process handle for pid %d: %w", pid, err)
	}

	// TODO: ensure it is the right process and not a reused PID?

	s.pidFd = fd

	err = s.watch()
	if err != nil {
		return nil, err
	}

	return s.quitCh, nil
}

func (s *forkStrategy) watch() error {
	done, err := s.pidfdWaiter.Add(s.pidFd)
	if err != nil {
		return fmt.Errorf("adding qemu pidfd to waiter: %w", err)
	}

	s.quitCh = done

	go func() {
		<-s.quitCh

		s.mu.Lock()
		defer s.mu.Unlock()

		err := syscall.Close(s.pidFd)
		if err != nil {
			s.logger.Logf("failed to close pidfd: %v", err)
		}
		s.pidFd = -1
	}()

	return nil
}

func (s *forkStrategy) Start(cmdWithArgs []string, fds []*os.File) (chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stderr, stdout *os.File
	var err error

	if s.stdoutFile != "" {
		stdout, err = os.Create(s.stdoutFile)
		if err != nil {
			return nil, fmt.Errorf("creating stdout log file: %w", err)
		}

		//goland:noinspection GoUnhandledErrorResult
		defer stdout.Close()
	}

	if s.stderrFile != "" {
		if s.stderrFile == s.stdoutFile {
			stderr = stdout
		} else {
			stderr, err = os.Create(s.stdoutFile)
			if err != nil {
				return nil, fmt.Errorf("creating stderr log file: %w", err)
			}

			//goland:noinspection GoUnhandledErrorResult
			defer stderr.Close()
		}
	}

	cmd := exec.Command(cmdWithArgs[0], cmdWithArgs[1:]...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.ExtraFiles = fds
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		PidFD:  &s.pidFd,
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("starting process (%s): %w", cmd, err)
	}

	err = s.watch()
	if err != nil {
		// TODO: the process is running but the watching failed, how do we handle this?
		return nil, err
	}

	return s.quitCh, nil
}

func (s *forkStrategy) IsRunning() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// TODO: should this do more? check if pidfd is readable to see if the process has exited and we somehow missed it?

	return s.pidFd != -1, nil
}

//goland:noinspection GoSnakeCaseUsage
const sys_pidfd_send_signal = 424

func (s *forkStrategy) Kill() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pidFd == -1 {
		return fmt.Errorf("not running")
	}

	_, _, errno := syscall.Syscall6(sys_pidfd_send_signal, uintptr(s.pidFd), uintptr(unix.SIGKILL), 0, uintptr(0), 0, 0)

	if errno != 0 {
		return fmt.Errorf("failed to kill: %w", errno)
	}
	return nil
}

func (s *forkStrategy) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var errs []error

	if s.pidFd != -1 {
		err := s.pidfdWaiter.Remove(s.pidFd)
		if err != nil {
			errs = append(errs, err)
		}

		err = syscall.Close(s.pidFd)
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
