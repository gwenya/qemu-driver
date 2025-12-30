package cmdBuilder

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
)

type cmdBuilder struct {
	path   string
	args   []string
	fds    []*os.File
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
	dir    string
	pidfd  *int
	sid    bool
}

type CmdBuilder interface {
	AddArgs(args ...string)
	AddFd(file *os.File) int
	ConnectStdin(stdin io.Reader)
	ConnectStdout(stdout io.Writer)
	ConnectStderr(stderr io.Writer)
	SetSession(session bool)
	SetPidFdReceiver(pidfd *int)
	SetDir(dir string)
	Build() *exec.Cmd
	BuildCtx(ctx context.Context) *exec.Cmd
	CloseFds()
}

func New(path string) CmdBuilder {
	return &cmdBuilder{
		path: path,
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

func (b *cmdBuilder) ConnectStdin(stdin io.Reader) {
	b.stdin = stdin
}

func (b *cmdBuilder) ConnectStdout(stdout io.Writer) {
	b.stdout = stdout
}

func (b *cmdBuilder) ConnectStderr(stderr io.Writer) {
	b.stderr = stderr
}

func (b *cmdBuilder) SetSession(session bool) {
	b.sid = session
}

func (b *cmdBuilder) SetDir(dir string) {
	b.dir = dir
}

func (b *cmdBuilder) SetPidFdReceiver(pidfd *int) {
	b.pidfd = pidfd
}

func (b *cmdBuilder) Build() *exec.Cmd {
	execCmd := exec.Command(b.path, b.args...)
	b.build(execCmd)
	return execCmd
}

func (b *cmdBuilder) BuildCtx(ctx context.Context) *exec.Cmd {
	execCmd := exec.CommandContext(ctx, b.path, b.args...)
	b.build(execCmd)
	return execCmd
}

func (b *cmdBuilder) CloseFds() {
	for _, fd := range b.fds {
		_ = fd.Close()
	}
}

func (b *cmdBuilder) build(execCmd *exec.Cmd) {
	execCmd.Stdout = b.stdout
	execCmd.Stderr = b.stderr
	execCmd.Stdin = b.stdin
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		PidFD:  b.pidfd,
	}
	execCmd.Dir = b.dir
	execCmd.ExtraFiles = b.fds
}
