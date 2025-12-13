package driver

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gwenya/qemu-driver/cmdBuilder"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/devices/chardev"
	"github.com/gwenya/qemu-driver/devices/pcie"
	"github.com/gwenya/qemu-driver/devices/storage"
	"github.com/gwenya/qemu-driver/machine"
	"github.com/gwenya/qemu-driver/qmp"
	"github.com/gwenya/qemu-driver/util"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

const (
	QemuPidFileName       = "qemu.pid"
	QemuQmpSocketFileName = "qmp.sock"
	RootDiskFileName      = "rootdisk.qcow2"
	ConfigFileName        = "qemu.conf"
	QemuStdErrFileName    = "stderr.log"
	QemuStdOutFileName    = "stdout.log"
	FirmwareFileName      = "firmware.fd"
	NvramFileName         = "nvram.fd"
)

type NetworkInterface interface {
	__networkInterfaceMarker()
}

type NetworkInterfaceBase struct {
	Id         uuid.UUID
	MacAddress net.HardwareAddr
}

func (PhysicalNetworkInterface) __networkInterfaceMarker() {}
func (TapNetworkInterface) __networkInterfaceMarker()      {}

type PhysicalNetworkInterface struct {
	NetworkInterfaceBase
	Name string
}

type TapNetworkInterface struct {
	NetworkInterfaceBase
	Name string
}

type Volume interface {
	__volumeMarker()
}

func (CephVolume) __volumeMarker() {}

type CephVolume struct {
	Id uuid.UUID
}

type MachineConfiguration struct {
	Id                 uuid.UUID
	StoragePath        string
	ImageSourcePath    string
	FirmwareSourcePath string
	NvramSourcePath    string
	CpuCount           uint32
	MemorySize         uint64
	DiskSize           uint64
	NetworkInterfaces  []NetworkInterface
	Volumes            []Volume
}

type Driver interface {
	Start() error
	Stop() error
	Reboot() error
	Scale(cpuCount uint32, memory uint64, disk uint64) error
	AttachNetworkInterface(net NetworkInterface) error
	DetachNetworkInterface(id uuid.UUID) error
	AttachVolume(volume Volume) error
	DetachVolume(id uuid.UUID) error
}

type driver struct {
	mu        sync.Mutex
	qemuPidFd int
	config    MachineConfiguration
	mon       qmp.Monitor
}

func New(config MachineConfiguration) (Driver, error) {
	d := &driver{
		config:    config,
		qemuPidFd: -1,
	}

	pidFilePath := d.filePath(QemuPidFileName)
	pidFileBytes, err := os.ReadFile(pidFilePath)
	if os.IsNotExist(err) {
		return d, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to open pid file %q: %w", pidFilePath, err)
	}

	pidInt64, err := strconv.ParseInt(strings.TrimSpace(string(pidFileBytes)), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pid file content %q: %w", string(pidFileBytes), err)
	}

	pid := int(pidInt64)

	pidfd, err := unix.PidfdOpen(pid, unix.PIDFD_NONBLOCK)
	if errors.Is(err, unix.ESRCH) {
		err := os.Remove(pidFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to remove old pid file %q: %w", pidFilePath, err)
		}

		return d, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to open process handle for pid %d: %w", pid, err)
	}

	cmdlineBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, fmt.Errorf("failed to read cmdline for pid %d: %w", pid, err)
	}

	cmdline := string(cmdlineBytes)

	if !strings.Contains(cmdline, config.Id.String()) {
		err := os.Remove(pidFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to remove old pid file %q: %w", pidFilePath, err)
		}

		return d, nil
	}

	// check if the process is still running, to prevent TOCTOU on the cmdline compare
	err = unix.PidfdSendSignal(pidfd, unix.Signal(0), nil, 0)
	if errors.Is(err, unix.ENOENT) {
		err := os.Remove(pidFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to remove old pid file %q: %w", pidFilePath, err)
		}

		return d, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to signal qemu process: %w", err)
	}

	d.qemuPidFd = pidfd
	d.startWatcher()

	return d, nil
}

func (d *driver) startWatcher() error {
	pidfd := d.qemuPidFd

	epfd, err := syscall.EpollCreate(1)
	if err != nil {
		return fmt.Errorf("creating epoll: %w", err)
	}

	err = syscall.EpollCtl(epfd, syscall.EPOLL_CTL_ADD, pidfd, &syscall.EpollEvent{
		Events: syscall.EPOLLIN,
	})
	if err != nil {
		return fmt.Errorf("configuring epoll: %w", err)
	}

	go func() {
		events := make([]syscall.EpollEvent, 1)

		for {
			ready, err := syscall.EpollWait(epfd, events, 1000)
			if err != nil {
				fmt.Printf("epoll_wait failed: %v", err) // TODO: use logger
				time.Sleep(time.Second * 1000)
			}

			if ready > 0 {
				break
			}
		}

		d.mu.Lock()
		d.qemuPidFd = -1
		d.signalExit()
		defer d.mu.Unlock()
	}()

	return nil
}

func (d *driver) signalExit() {

}

func (d *driver) copyIfNotExists(srcPath string, dstFileName string) error {
	dstPath := d.filePath(dstFileName)
	exists, err := util.FileExists(dstPath)
	if err != nil {
		return fmt.Errorf("stat on %s: %w", dstPath, err)
	}

	if !exists {
		err := util.CopyFile(srcPath, dstPath)
		if err != nil {
			return fmt.Errorf("copying %s to %s: %w", srcPath, dstPath, err)
		}
	}

	return nil
}

func (d *driver) ensureRootDisk() error {
	dstPath := d.filePath(RootDiskFileName)
	exists, err := util.FileExists(dstPath)
	if err != nil {
		return fmt.Errorf("stat on %s: %w", dstPath, err)
	}

	if !exists {
		cmd := exec.Command("qemu-img", "convert", "-f", "qcow2", "-O", "raw", d.config.ImageSourcePath, dstPath)
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("converting image: %w", err)
		}

		var diskKiB uint64

		if d.config.DiskSize%4096 != 0 {
			diskKiB = ((d.config.DiskSize / 4096) + 1) * 4
		} else {
			diskKiB = d.config.DiskSize / 1024
		}

		cmd = exec.Command("qemu-img", "resize", dstPath, fmt.Sprintf("%dK", diskKiB))
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("resizing image: %w", err)
		}
	}

	return nil
}

func (d *driver) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.qemuPidFd != -1 {
		return fmt.Errorf("VM is already running")
	}

	err := d.ensureRootDisk()
	if err != nil {
		return fmt.Errorf("ensuring root disk: %w", err)
	}

	err = d.copyIfNotExists(d.config.FirmwareSourcePath, FirmwareFileName)
	if err != nil {
		return fmt.Errorf("ensuring firmware file: %w", err)
	}

	err = d.copyIfNotExists(d.config.NvramSourcePath, NvramFileName)
	if err != nil {
		return fmt.Errorf("ensuring nvram file: %w", err)
	}

	desc := machine.Description{}

	desc.Cpu(1, int(d.config.CpuCount), int(d.config.CpuCount))

	memoryMiB := int(d.config.MemorySize / 1024 / 1024)

	if d.config.MemorySize%(1024*1024) != 0 {
		memoryMiB += 1
	}

	desc.Memory(memoryMiB, 64*1024) // TODO: configurable max size

	desc.Monitor(d.filePath(QemuQmpSocketFileName))

	desc.Scsi().AddDisk(storage.NewImageDrive("rootdisk", d.filePath(RootDiskFileName)))

	for _, volume := range d.config.Volumes {
		switch v := volume.(type) {
		case CephVolume:
			_ = v
			desc.Scsi().AddDisk(storage.NewRbdDrive()) // TODO: volume config
		}
	}

	for _, networkInterface := range d.config.NetworkInterfaces {
		switch n := networkInterface.(type) {
		case TapNetworkInterface:
			desc.Pcie().AddDevice(pcie.NewTapNetworkDevice("tap-"+n.Id.String(), n.Name, int(d.config.CpuCount)))
		case PhysicalNetworkInterface:
			desc.Pcie().AddDevice(pcie.NewPhysicalNetworkDevice("phys-"+n.Id.String(), n.Name))
		}
	}

	desc.Pcie().AddDevice(pcie.NewBalloon("balloon"))
	desc.Pcie().AddDevice(pcie.NewKeyboard("keyboard"))
	desc.Pcie().AddDevice(pcie.NewTablet("tablet"))
	desc.Pcie().AddDevice(pcie.NewVga("vga", pcie.StdVga))

	desc.AddChardev(chardev.NewStdio("console", true)) // TODO: actually want a ringbuf here, but this is easier for debugging

	config, hotplugDevices := desc.BuildConfig()

	configFilePath := d.filePath(ConfigFileName)

	err = os.WriteFile(configFilePath, []byte(config.ToString()), 0o644)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	builder := cmdBuilder.New("/usr/sbin/qemu-system-x86_64")
	defer builder.CloseFds()

	builder.AddArgs(
		"-S",
		"-uuid", d.config.Id.String(),
		//"-nographic",
		"-display", "gtk",
		"-nodefaults",
		"-no-user-config",
		"-serial", "chardev:console",
		"-readconfig", configFilePath,
		"-machine", "q35",
		"-pidfile", d.filePath(QemuPidFileName),
	)

	stdout, err := os.Create(d.filePath(QemuStdOutFileName))
	if err != nil {
		return fmt.Errorf("creating stdout log file: %w", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer stdout.Close()

	stderr, err := os.Create(d.filePath(QemuStdErrFileName))
	if err != nil {
		return fmt.Errorf("creating stderr log file: %w", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer stderr.Close()

	builder.ConnectStderr(stderr)
	builder.ConnectStdout(stdout)

	builder.SetSession(true)

	var pidfd int
	builder.SetPidFdReceiver(&pidfd)

	cmd := builder.Build()

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("starting qemu process: %w", err)
	}

	d.qemuPidFd = pidfd
	d.startWatcher()

	_, err = d.connectMonitor()
	if err != nil {
		return err
	}

	return d.postStartConfig(hotplugDevices)
}

func (d *driver) postStartConfig(devices []devices.HotplugDevice) error {
	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	var hotplugErrors []error

	for _, device := range devices {
		err := device.Plug(mon)
		if err != nil {
			hotplugErrors = append(hotplugErrors, err)
		}
	}

	if hotplugErrors != nil {
		return fmt.Errorf("hotplugging: %w", errors.Join(hotplugErrors...))
	}

	err = mon.Continue()
	if err != nil {
		return fmt.Errorf("resuming VM execution: %w", err)
	}

	return nil
}

func (d *driver) connectMonitor() (qmp.Monitor, error) {
	if d.mon != nil {
		// TODO: check if the monitor was disconnected, by pinging or something, reconnect if necessary
		return d.mon, nil
	}

	var mon qmp.Monitor
	var err error

	for {
		mon, err = qmp.Connect(d.filePath(QemuQmpSocketFileName))
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ECONNREFUSED) {
			fmt.Println("qmp socket is not ready yet, retrying in a second")
			time.Sleep(time.Second * 1)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("connecting to qmp monitor: %w", err)
		}

		break
	}

	d.mon = mon
	return mon, nil
}

func (d *driver) filePath(name string) string {
	return path.Join(d.config.StoragePath, name)
}

func (d *driver) Stop() error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) Reboot() error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) Scale(cpuCount uint32, memory uint64, disk uint64) error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) AttachNetworkInterface(net NetworkInterface) error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) DetachNetworkInterface(id uuid.UUID) error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) AttachVolume(volume Volume) error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) DetachVolume(id uuid.UUID) error {
	//TODO implement me
	panic("implement me")
}
