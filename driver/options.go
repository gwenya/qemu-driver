package driver

import (
	"path"

	"github.com/google/uuid"
	"github.com/gwenya/qemu-driver/execution"
	"github.com/gwenya/qemu-driver/pidfd"
	"github.com/gwenya/qemu-driver/systemd"
)

type Option interface {
	apply(*driver) error
	priority() int
}

type driverOption struct {
	fn   func(*driver) error
	prio int
}

func (o *driverOption) apply(d *driver) error {
	return o.fn(d)
}

func (o *driverOption) priority() int {
	return o.prio
}

func WithSystemId(id uuid.UUID) Option {
	return &driverOption{
		fn: func(d *driver) error {
			d.systemId = id
			return nil
		},
		prio: 0,
	}
}

func WithStorageDirectory(dir string) Option {
	return &driverOption{
		fn: func(d *driver) error {
			d.storageDirectory = dir
			return nil
		},
		prio: 0,
	}
}

func WithRuntimeDirectory(dir string) Option {
	return &driverOption{
		fn: func(d *driver) error {
			d.runtimeDirectory = dir
			return nil
		},
		prio: 0,
	}
}

func WithQemuPath(path string) Option {
	return &driverOption{
		fn: func(d *driver) error {
			d.qemuPath = path
			return nil
		},
		prio: 0,
	}
}

func WithLogger(logger Logger) Option {
	return &driverOption{
		fn: func(d *driver) error {
			d.logger = logger
			return nil
		},
		prio: 0,
	}
}

func WithForkStrategy(pidFdWaiter pidfd.Waiter) Option {
	return &driverOption{
		fn: func(d *driver) error {
			executionStrategy, err := execution.NewForkStrategy(execution.ForkOptions{
				PidFilePath:    path.Join(d.runtimePath(QemuPidFileName)),
				StdoutFilePath: path.Join(d.storagePath(QemuStdOutFileName)),
				StderrFilePath: path.Join(d.storagePath(QemuStdErrFileName)),
				Logger:         d.logger,
				PidFdWaiter:    pidFdWaiter,
			})
			if err != nil {
				return err
			}

			d.executionStrategy = executionStrategy
			return nil
		},
		prio: 1,
	}
}

type SystemdStrategyOptions struct {
	UnitNamePrefix string
	SELinuxContext string
}

func WithSystemdStrategy(opts SystemdStrategyOptions, sdWaiter systemd.Waiter) Option {
	return &driverOption{
		fn: func(d *driver) error {
			executionStrategy, err := execution.NewSystemdStrategy(execution.SystemdOptions{
				UnitNameFilePath: path.Join(d.runtimePath(QemuUnitNameFileName)),
				UnitNamePrefix:   opts.UnitNamePrefix,
				SELinuxContext:   opts.SELinuxContext,
				Logger:           d.logger,
				SystemdWaiter:    sdWaiter,
			})
			if err != nil {
				return err
			}

			d.executionStrategy = executionStrategy
			return nil
		},
		prio: 1,
	}
}

func WithEventChannel(ch chan Event) Option {
	return &driverOption{
		fn: func(d *driver) error {
			d.events.Subscribe(ch)
			return nil
		},
	}
}
