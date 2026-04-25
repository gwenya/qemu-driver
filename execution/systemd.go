package execution

import (
	"context"
	"fmt"
	"os"

	sd "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/google/uuid"
	"github.com/gwenya/qemu-driver/systemd"
)

type systemdStrategy struct {
	unitName       string
	unitNameFile   string
	unitNamePrefix string
	selinuxContext string
	sdConn         *sd.Conn
	sdConnOwned    bool
	sdWaiter       systemd.Waiter
	sdWaiterOwned  bool
	logger         Logger
	chIn           chan struct{}
	chOut          chan struct{}
}

type SystemdOptions struct {
	UnitNameFilePath  string
	UnitNamePrefix    string
	SELinuxContext    string
	Logger            Logger
	SystemdWaiter     systemd.Waiter
	SystemdConnection *sd.Conn
}

func NewSystemdStrategy(opts SystemdOptions) (Strategy, error) {
	var err error

	sdConn := opts.SystemdConnection
	sdConnOwned := false
	if sdConn == nil {
		sdConn, err = sd.NewSystemConnectionContext(context.Background())
		if err != nil {
			return nil, err
		}

		sdConnOwned = true
	}

	sdWaiter := opts.SystemdWaiter
	sdWaiterOwned := false
	if sdWaiter == nil {
		sdWaiter, err = systemd.NewWaiter(opts.UnitNamePrefix)
		if err != nil {
			return nil, fmt.Errorf("creating systemd waiter: %w", err)
		}

		sdWaiterOwned = true
	}

	return &systemdStrategy{
		unitNameFile:   opts.UnitNameFilePath,
		unitNamePrefix: opts.UnitNamePrefix,
		selinuxContext: opts.SELinuxContext,
		sdConn:         sdConn,
		sdConnOwned:    sdConnOwned,
		sdWaiter:       sdWaiter,
		sdWaiterOwned:  sdWaiterOwned,
		logger:         opts.Logger,
	}, nil
}

func (s *systemdStrategy) FindRunning() (chan struct{}, error) {
	unitNameBytes, err := os.ReadFile(s.unitNameFile)
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("opening unit name file: %w", err)
	}

	unitName := string(unitNameBytes)

	s.chIn, err = s.sdWaiter.Add(unitName)
	if err != nil {
		return nil, err
	}

	unitProps, err := s.sdConn.GetUnitPropertiesContext(context.TODO(), unitName)
	if err != nil {
		return nil, err
	}

	loadErr := unitProps["LoadError"].([]interface{})[0].(string)
	inactiveEnterTimestamp := unitProps["InactiveEnterTimestamp"].(uint64)

	running := loadErr == "" && inactiveEnterTimestamp == 0

	if running {
		s.chOut = make(chan struct{})
		go s.watch()
		s.unitName = unitName
		return s.chOut, nil
	}

	// TODO: properly check if it's the not exists error, which is currently the only error that `Remove` can return
	_ = s.sdWaiter.Remove(unitName)

	s.chIn = nil
	s.chOut = nil
	s.unitName = ""

	err = os.Remove(s.unitNameFile)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return nil, nil
}

type ExtraFd struct {
	Fd   dbus.UnixFD
	Name string
}

func (s *systemdStrategy) Start(cmd []string, fds []*os.File) (chan struct{}, error) {
	unitName := fmt.Sprintf("%s%s.service", s.unitNamePrefix, uuid.New())

	err := os.WriteFile(s.unitNameFile, []byte(unitName), 0644)
	if err != nil {
		return nil, fmt.Errorf("writing unit name file: %w", err)
	}

	extraFds := make([]ExtraFd, 0, len(fds))
	for _, fd := range fds {
		extraFds = append(extraFds, ExtraFd{
			Fd:   dbus.UnixFD(fd.Fd()),
			Name: "extra-fd",
		})
	}

	ch := make(chan string)

	props := []sd.Property{
		sd.PropDescription(fmt.Sprintf("Beanstack VM instance <TODO>")),
		sd.PropExecStart(cmd, false),
		{
			Name:  "ExtraFileDescriptors",
			Value: dbus.MakeVariant(extraFds),
		},
	}
	if s.selinuxContext != "" {
		props = append(props, sd.Property{
			Name:  "SELinuxContext",
			Value: dbus.MakeVariant(s.selinuxContext),
		})
	}

	_, err = s.sdConn.StartTransientUnitContext(context.TODO(), unitName, "replace", props, ch)
	if err != nil {
		return nil, fmt.Errorf("starting transient unit: %w", err)
	}

	result := <-ch
	if result != "done" {
		return nil, fmt.Errorf("transient unit failed to start: %s", result)
	}

	s.chIn, err = s.sdWaiter.Add(unitName)
	if err != nil {
		return nil, fmt.Errorf("adding transient unit to waiter: %w", err)
	}

	s.chOut = make(chan struct{})
	go s.watch()

	s.unitName = unitName

	return s.chOut, nil
}

func (s *systemdStrategy) IsRunning() (bool, error) {
	return s.unitName != "", nil
}

func (s *systemdStrategy) Kill() error {
	ch := make(chan string)
	_, err := s.sdConn.StopUnitContext(context.TODO(), s.unitName, "replace", ch)
	if err != nil {
		return fmt.Errorf("stopping unit: %w", err)
	}

	result := <-ch
	if result != "done" {
		return fmt.Errorf("failed to stop unit: %s", result)
	}

	return nil
}

func (s *systemdStrategy) Close() error {
	if s.sdConnOwned {
		s.sdConn.Close()
	}

	if s.sdWaiterOwned {
		return s.sdWaiter.Close()
	}

	return nil
}

func (s *systemdStrategy) watch() {
	<-s.chIn
	s.unitName = ""
	close(s.chOut)
}
