package execution

import (
	"io"
	"os"
)

type Strategy interface {
	FindRunning() (chan struct{}, error)
	Start(cmd []string, fds []*os.File) (chan struct{}, error)
	IsRunning() (bool, error)
	Kill() error

	io.Closer
}

type Logger interface {
	Logf(format string, v ...interface{})
}
