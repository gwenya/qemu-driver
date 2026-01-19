package cmdBuilder

import (
	"os"
)

type cmdBuilder struct {
	args []string
	fds  []*os.File
}

type CmdBuilder interface {
	AddArgs(args ...string)
	AddFd(file *os.File) int

	GetCommand() []string
	GetFds() []*os.File
	CloseFds()
}

func New(path string) CmdBuilder {
	return &cmdBuilder{
		args: []string{path},
	}
}

func (b *cmdBuilder) AddArgs(args ...string) {
	b.args = append(b.args, args...)
}

func (b *cmdBuilder) AddFd(file *os.File) int {
	fd := 3 + len(b.fds)
	b.fds = append(b.fds, file)
	return fd
}

func (b *cmdBuilder) GetCommand() []string {
	return b.args
}

func (b *cmdBuilder) GetFds() []*os.File {
	return b.fds
}

func (b *cmdBuilder) CloseFds() {
	for _, fd := range b.fds {
		_ = fd.Close()
	}
}
