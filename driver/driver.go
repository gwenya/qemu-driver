package driver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
	"github.com/gwenya/qemu-driver/pidfd"
	"github.com/kdomanski/iso9660"
	"golang.org/x/sys/unix"

	"github.com/gwenya/qemu-driver/cmdBuilder"
	"github.com/gwenya/qemu-driver/devices/chardev"
	"github.com/gwenya/qemu-driver/devices/pcie"
	"github.com/gwenya/qemu-driver/devices/storage"
	"github.com/gwenya/qemu-driver/machine"
	"github.com/gwenya/qemu-driver/qmp"
	"github.com/gwenya/qemu-driver/util"
)

type Driver interface {
	Create(config CreateOptions) error
	Start(config StartOptions) error
	Stop() error
	Reboot() error
	GetStatus() Status

	GetCPUs() (uint32, error)
	SetCPUs(count uint32) error

	GetMemory() (uint64, error)
	SetMemory(bytes uint64) error

	GetNetworkAdapterNames() ([]string, error)
	AttachNetworkAdapter(adapter NetworkAdapter) error
	DetachNetworkAdapter(name string) error

	GetVolumeIdentifiers() ([]DiskIdentifier, error)
	AttachVolume(volume Volume) error
	DetachVolume(id DiskIdentifier) error

	io.Closer
}

type driver struct {
	logger           Logger
	qemuPath         string
	systemId         uuid.UUID
	storageDirectory string
	runtimeDirectory string

	mu               sync.Mutex
	qemuPidFd        int
	pidfdWaiter      pidfd.Waiter
	pidfdWaiterOwned bool
	mon              qmp.Monitor
	cancelWatcher    context.CancelFunc
}

func New(opts Options) (Driver, error) {
	pidfdWaiter := opts.PidFdWaiter
	pidfdWaiterOwned := false
	if pidfdWaiter == nil {
		pidfdWaiterOwned = true
		var err error
		pidfdWaiter, err = pidfd.NewWaiter()
		if err != nil {
			return nil, fmt.Errorf("creating pidfd waiter: %w", err)
		}
	}
	d := &driver{
		systemId:         opts.SystemId,
		qemuPath:         opts.QemuPath,
		logger:           opts.Logger,
		qemuPidFd:        -1,
		storageDirectory: opts.StorageDirectory,
		runtimeDirectory: opts.RuntimeDirectory,
		pidfdWaiter:      pidfdWaiter,
		pidfdWaiterOwned: pidfdWaiterOwned,
	}

	pidFilePath := d.runtimePath(QemuPidFileName)
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

	cmdline, err := util.GetCmdline(pid)
	if err != nil {
		return nil, fmt.Errorf("getting cmdline of running process: %w", err)
	}

	// TODO: this feels very brittle, maybe instead/first make an attempt to connect to the monitor?
	if len(cmdline) < 3 || cmdline[0] != d.qemuPath || cmdline[1] != "-uuid" || cmdline[2] != d.systemId.String() {
		err := os.Remove(pidFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to remove old pid file %q: %w", pidFilePath, err)
		}

		return d, nil
	}

	d.qemuPidFd = pidfd

	err = d.startWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating qemu process watcher: %w", err)
	}

	return d, nil
}

func (d *driver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancelWatcher != nil {
		d.cancelWatcher()
	}

	var errs []error

	if d.qemuPidFd != -1 {
		err := d.pidfdWaiter.Remove(d.qemuPidFd)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if d.pidfdWaiterOwned {
		err := d.pidfdWaiter.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (d *driver) startWatcher() error {
	done, err := d.pidfdWaiter.Add(d.qemuPidFd)
	if err != nil {
		return fmt.Errorf("adding qemu pidfd to waiter: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	d.cancelWatcher = cancel

	go func() {
		select {
		case <-done:
		case <-ctx.Done():
		}

		d.mu.Lock()
		defer d.mu.Unlock()

		if d.mon != nil {
			err := d.mon.Disconnect()
			if err != nil {
				d.logger.Logf("closing qmp socket failed: %v\n", err)
			}
		}

		err := syscall.Close(d.qemuPidFd)
		if err != nil {
			d.logger.Logf("failed to close pidfd: %v", err)
		}

		d.mon = nil
		d.qemuPidFd = -1
		d.cancelWatcher = nil

		if ctx.Err() == nil {
			d.onQemuProcessExit()
		}
	}()

	return nil
}

func (d *driver) onQemuProcessExit() {
	d.logger.Logf("qemu process stopped")
}

func (d *driver) storagePath(name StorageFilename) string {
	return path.Join(d.storageDirectory, string(name))
}

func (d *driver) runtimePath(name RuntimeFilename) string {
	return path.Join(d.runtimeDirectory, string(name))
}

func (d *driver) createRootDisk(sourcePath string, sourceFormat string) error {
	rootDiskPath := d.storagePath(RootDiskFileName)
	err := util.RemoveIfExists(rootDiskPath)
	if err != nil {
		return fmt.Errorf("deleting existing root disk: %w", err)
	}

	args := []string{"convert"}

	if sourceFormat != "" {
		args = append(args, "-f", sourceFormat)
	}

	args = append(args, "-O", "raw", sourcePath, rootDiskPath)

	cmd := exec.Command("qemu-img", args...)
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("converting image (%s): %w", cmd, err)
	}

	return nil
}

func (d *driver) Create(opts CreateOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// we specifically check for the pidfd here rather than using GetStatus() because it matters
	// whether the qemu process is running, not whether the VM is in a running state.
	if d.qemuPidFd != -1 {
		return fmt.Errorf("qemu process is running")
	}

	err := d.createRootDisk(opts.ImageSourcePath, opts.ImageSourceFormat)
	if err != nil {
		return fmt.Errorf("creating root disk: %w", err)
	}

	err = os.WriteFile(d.storagePath(CreatedFlagFileName), make([]byte, 0), 0o644)
	if err != nil {
		return fmt.Errorf("creating flag file: %w", err)
	}

	return nil
}

func (d *driver) Start(opts StartOptions) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.getStatus() == Running {
		return fmt.Errorf("VM is already running")
	}

	configFilePath := d.storagePath(ConfigFileName)
	err := util.RemoveIfExists(configFilePath)
	if err != nil {
		return fmt.Errorf("removing old config file: %w", err)
	}

	err = util.RemoveIfExists(d.runtimePath(QemuQmpSocketFileName))
	if err != nil {
		return fmt.Errorf("removing old qmp socket: %w", err)
	}

	err = util.RemoveIfExists(d.runtimePath(ConsoleSocketFileName))
	if err != nil {
		return fmt.Errorf("removing old console socket: %w", err)
	}

	err = util.RemoveIfExists(d.runtimePath(QemuPidFileName))
	if err != nil {
		return fmt.Errorf("removing old pid file: %w", err)
	}

	if opts.DiskSize != 0 {
		err := d.resizeRootdisk(opts.DiskSize)
		if err != nil {
			return fmt.Errorf("resizing rootdisk: %w", err)
		}
	}

	builder := cmdBuilder.New(d.qemuPath)
	defer builder.CloseFds()

	builder.AddArgs(
		"-uuid", d.systemId.String(),
		"-S",
		"-nographic",
		//"-display", "gtk",
		"-nodefaults",
		"-no-user-config",
		"-serial", "chardev:console",
		"-readconfig", configFilePath,
		"-machine", "q35",
		"-pidfile", d.runtimePath(QemuPidFileName),
	)

	desc := machine.Description{}

	desc.Cpu(int(opts.CpuCount), 8) // TODO: configureable max cpus

	memoryMiB := int(opts.MemorySize / 1024 / 1024)

	if opts.MemorySize%(1024*1024) != 0 {
		memoryMiB += 1
	}

	desc.Memory(memoryMiB, 64*1024) // TODO: configurable max size

	monitorSocketFile, err := d.makeUnixListener(d.runtimePath(QemuQmpSocketFileName))
	if err != nil {
		return fmt.Errorf("creating monitor socket: %w", err)
	}

	desc.AddChardev(chardev.NewSocket("monitor", chardev.SocketOpts{
		Unix: chardev.SocketOptsUnix{
			Fd: builder.AddFd(monitorSocketFile),
		},
		Server: true,
	}))
	desc.Monitor("monitor")

	desc.Pcie().AddDevice(pcie.NewBalloon("balloon"))
	desc.Pcie().AddDevice(pcie.NewKeyboard("keyboard"))
	desc.Pcie().AddDevice(pcie.NewTablet("tablet"))
	desc.Pcie().AddDevice(pcie.NewVga("vga", pcie.StdVga))

	desc.AddChardev(chardev.NewRingbuf("console-ringbuf", 4096))

	consoleSocketFile, err := d.makeUnixListener(d.runtimePath(ConsoleSocketFileName))
	if err != nil {
		return fmt.Errorf("creating console socket: %w", err)
	}

	desc.AddChardev(chardev.NewSocket("console-socket", chardev.SocketOpts{
		Unix: chardev.SocketOptsUnix{
			Fd: builder.AddFd(consoleSocketFile),
		},
		Server: true,
		Wait:   false,
	}))

	desc.AddChardev(chardev.NewHub("console", "console-ringbuf", "console-socket"))

	desc.Scsi().AddDisk(storage.NewImageDrive("rootdisk", d.storagePath(RootDiskFileName)))

	if (opts.CloudInit != CloudInit{}) {
		err := d.generateCloudInitIso(opts.CloudInit)
		if err != nil {
			return fmt.Errorf("generating cloud-init iso: %w", err)
		}

		desc.Scsi().AddDisk(storage.NewCdromDrive("cloud-init-cidata", d.storagePath(CloudInitIsoFile)))
	}

	for _, volume := range opts.Volumes {
		switch opts := volume.options.(type) {
		case cephVolumeOpts:
			// TODO: pass vendor and model, and more rbd options
			desc.Scsi().AddDisk(storage.NewRbdDrive(volume.id.Serial, opts.pool, opts.name))
		default:
			panic("not implemented")
		}
	}

	for _, networkAdapter := range opts.NetworkAdapters {
		switch opts := networkAdapter.options.(type) {
		case tapNetworkAdapterOptions:
			desc.Pcie().AddDevice(pcie.NewTapNetworkDevice("tap-"+networkAdapter.name, networkAdapter.name, opts.macAddress))
		default:
			panic("not implemented")
		}
	}

	if opts.VsockCid != 0 {
		vsockFile, err := openVsock(opts.VsockCid)
		if err != nil {
			return fmt.Errorf("allocating vsock cid %d: %w", opts.VsockCid, err)
		}

		vsockFd := builder.AddFd(vsockFile)
		desc.Pcie().AddDevice(pcie.NewVsock("vsock", opts.VsockCid, vsockFd))
	}

	config, hotplugDevices := desc.BuildConfig()

	configFile, err := os.Create(configFilePath)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}

	_, err = config.WriteTo(configFile)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	stdout, err := os.Create(d.storagePath(QemuStdOutFileName))
	if err != nil {
		return fmt.Errorf("creating stdout log file: %w", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer stdout.Close()

	stderr, err := os.Create(d.storagePath(QemuStdErrFileName))
	if err != nil {
		return fmt.Errorf("creating stderr log file: %w", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer stderr.Close()

	builder.ConnectStderr(stderr)
	builder.ConnectStdout(stdout)

	builder.SetSession(true)

	var pidfdReceiver int
	builder.SetPidFdReceiver(&pidfdReceiver)

	cmd := builder.Build()

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("starting qemu process (%s): %w", cmd, err)
	}

	d.qemuPidFd = pidfdReceiver
	err = d.startWatcher()
	if err != nil {
		return fmt.Errorf("starting qemu process watcher: %w", err)
	}

	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	var hotplugErrors []error

	for _, device := range hotplugDevices {
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
		return fmt.Errorf("starting VM execution: %w", err)
	}

	return nil
}

func (d *driver) makeUnixListener(path string) (*os.File, error) {
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("creating listener: %w", err)
	}

	err = os.Chmod(path, 0777)
	if err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("changing permissions on socket path: %w", err)
	}

	socketFile, err := listener.(*net.UnixListener).File()
	if err != nil {
		return nil, fmt.Errorf("getting fd from listener: %w", err)
	}

	return socketFile, nil
}

func (d *driver) connectMonitor() (qmp.Monitor, error) {
	if d.mon != nil {
		_, err := d.mon.Status()
		if err == nil {
			return d.mon, nil
		}

		d.logger.Logf("connection to qmp socket is broken, reconnecting")
		_ = d.mon.Disconnect()
	}

	mon, err := qmp.Connect(d.runtimePath(QemuQmpSocketFileName))
	if err != nil {
		return nil, fmt.Errorf("connecting to qmp monitor: %w", err)
	}

	d.mon = mon
	return mon, nil
}

func (d *driver) generateCloudInitIso(cidata CloudInit) error {
	hash := cidata.Hash()
	hashFilePath := d.storagePath(CloudInitHashFile)

	exists, err := util.FileExists(hashFilePath)
	if err != nil {
		return fmt.Errorf("checking cloud init hash file: %w", err)
	}

	if exists {
		oldHash, err := os.ReadFile(hashFilePath)
		if err != nil {
			return fmt.Errorf("reading cloud init hash file: %w", err)
		}

		if string(oldHash) == hash {
			return nil
		}
	}

	writer, err := iso9660.NewWriter()
	if err != nil {
		return fmt.Errorf("creating iso writer: %w", err)
	}

	defer func() {
		err := writer.Cleanup()
		if err != nil {
			d.logger.Logf("failed to clean up iso writer: %v", err)
		}
	}()

	err = writer.AddFile(strings.NewReader(cidata.Meta), "meta-data")
	if err != nil {
		return fmt.Errorf("adding meta-data: %w", err)
	}

	err = writer.AddFile(strings.NewReader(cidata.User), "user-data")
	if err != nil {
		return fmt.Errorf("adding user-data: %w", err)
	}

	if cidata.Network != "" {
		err = writer.AddFile(strings.NewReader(cidata.Network), "network-config")
		if err != nil {
			return fmt.Errorf("adding network-config: %w", err)
		}
	}

	if cidata.Vendor != "" {
		err = writer.AddFile(strings.NewReader(cidata.Vendor), "vendor-data")
		if err != nil {
			return fmt.Errorf("adding vendor-data: %w", err)
		}
	}

	isoFile, err := os.OpenFile(d.storagePath(CloudInitIsoFile), os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0600)
	if err != nil {
		log.Fatalf("failed to create file: %s", err)
	}
	defer func() {
		err := isoFile.Close()
		if err != nil {
			d.logger.Logf("failed to close iso file: %v", err)
		}
	}()

	err = writer.WriteTo(isoFile, "cidata")
	if err != nil {
		return fmt.Errorf("writing iso file: %w", err)
	}

	err = os.WriteFile(hashFilePath, []byte(hash), 0600)
	if err != nil {
		return fmt.Errorf("writing cloud init hash file: %w", err)
	}

	return nil

}

func (d *driver) resizeRootdisk(size uint64) error {
	stat, err := os.Stat(d.storagePath(RootDiskFileName))
	if err != nil {
		return fmt.Errorf("stat %q: %w", d.storagePath(RootDiskFileName), err)
	}

	desiredSize := size

	if size%4096 != 0 {
		desiredSize = ((size / 4096) + 1) * 4096
	}

	currentSize := uint64(stat.Size())

	if currentSize == desiredSize {
		return nil
	}

	sizeKiB := desiredSize / 1024

	cmd := exec.Command("qemu-img", "resize", "-f", "raw", d.storagePath(RootDiskFileName), fmt.Sprintf("%dK", sizeKiB))
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("resizing image (%s): %w", cmd, err)
	}

	return nil
}

func (d *driver) Stop() error {
	// TODO: if it fails, kill the qemu process

	d.mu.Lock()
	defer d.mu.Unlock()
	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	err = mon.Quit()
	if err != nil {
		return fmt.Errorf("stopping VM: %w", err)
	}

	err = d.mon.Disconnect()
	if err != nil {
		d.logger.Logf("failed to close qmp: %v", err)
	}

	d.mon = nil

	return nil
}

func (d *driver) Reboot() error {
	//TODO implement me
	panic("implement me")
}

func (d *driver) getStatus() Status {
	// TODO: proper state logic for starting, stopping and restarting states
	if d.qemuPidFd == -1 {
		isCreated, err := util.FileExists(d.storagePath(CreatedFlagFileName))
		if err != nil {
			d.logger.Logf("failed to check creation flag existence: %v\n", err)
			return Unknown
		}

		if !isCreated {
			return Uninitialized
		}

		return Stopped
	}

	mon, err := d.connectMonitor()
	if err != nil {
		d.logger.Logf("failed to connect qmp monitor: %v\n", err)
		return Unknown
	}

	qemuStatus, err := mon.Status()
	if err != nil {
		d.logger.Logf("failed to query qemu status: %v\n", err)
		return Unknown
	}

	return mapQemuStatus(qemuStatus)
}

func (d *driver) GetStatus() Status {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.getStatus()
}

func (d *driver) GetCPUs() (uint32, error) {
	mon, err := d.connectMonitor()
	if err != nil {
		return 0, err
	}

	cpus, err := mon.QueryCPUs()
	if err != nil {
		return 0, err
	}

	return uint32(len(cpus)), nil
}

func (d *driver) SetCPUs(count uint32) error {
	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	cpus, err := mon.QueryHotpluggableCPUs()
	if err != nil {
		return err
	}

	var availableCPUs []qmp.HotpluggableCpu
	var pluggedCPUs []qmp.HotpluggableCpu

	for _, cpu := range cpus {
		if cpu.QomPath == "" {
			availableCPUs = append(availableCPUs, cpu)
		} else {
			pluggedCPUs = append(pluggedCPUs, cpu)
		}
	}

	plugged := len(pluggedCPUs)
	available := len(availableCPUs)
	needed := int(count) - plugged

	if needed == 0 {
		return nil
	}

	if needed < 0 {
		return newRestartRequiredErr("hot-unplugging CPUs is not supported")
	}

	if needed > available {
		return newRestartRequiredErr(fmt.Sprintf("not enough CPU hotplug slots"))
	}

	slices.SortFunc(availableCPUs, func(a, b qmp.HotpluggableCpu) int {
		if a.Props.SocketId != b.Props.SocketId {
			return a.Props.SocketId - b.Props.SocketId
		}

		if a.Props.CoreId != b.Props.CoreId {
			return a.Props.CoreId - b.Props.CoreId
		}

		return a.Props.ThreadId - b.Props.ThreadId
	})

	for i := range needed {
		cpu := availableCPUs[i]
		id := fmt.Sprintf("cpu-%d-%d-%d", cpu.Props.SocketId, cpu.Props.CoreId, cpu.Props.ThreadId)
		err := mon.AddDevice(map[string]any{
			"id":        id,
			"driver":    cpu.Type,
			"socket-id": cpu.Props.SocketId,
			"core-id":   cpu.Props.CoreId,
			"thread-id": cpu.Props.ThreadId,
		})
		if err != nil {
			return fmt.Errorf("CPU hotplug incomplete: %w", err)
		}
	}

	return nil
}

func (d *driver) GetMemory() (uint64, error) {
	mon, err := d.connectMonitor()
	if err != nil {
		return 0, err
	}

	memory, err := mon.QueryMemorySummary()
	if err != nil {
		return 0, err
	}

	return memory.Base + memory.Hotplugged, nil
}

func (d *driver) SetMemory(bytes uint64) error {
	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	memory, err := mon.QueryMemorySummary()
	if err != nil {
		return err
	}

	total := memory.Base + memory.Hotplugged
	if bytes == total {
		return nil
	}

	if bytes < total {
		return newRestartRequiredErr("hot-unplugging memory is not supported")
	}

	devices, err := mon.QueryMemoryDevices()
	if err != nil {
		return err
	}

	highestIdx := 0
	for _, device := range devices {
		if strings.HasPrefix(device.Data.Id, "dimm") {
			idx, err := strconv.Atoi(strings.TrimPrefix(device.Data.Id, "dimm"))
			if err == nil {
				highestIdx = max(highestIdx, idx)
			}
		}
	}

	memId := fmt.Sprintf("mem%d", highestIdx+1)
	deviceId := fmt.Sprintf("dimm%d", highestIdx+1)

	err = mon.AddMemoryBackend(memId, bytes-total)
	if err != nil {
		return err
	}

	err = mon.AddDevice(map[string]any{
		"driver": "pc-dimm",
		"memdev": memId,
		"id":     deviceId,
	})

	if err != nil {
		err2 := mon.RemoveMemoryBackend(memId)
		if err2 != nil {
			d.logger.Logf("failed to remove memory backend %s: %v", memId, err)
		}
		return err
	}

	return nil
}

func (d *driver) GetNetworkAdapterNames() ([]string, error) {
	mon, err := d.connectMonitor()
	if err != nil {
		return nil, err
	}

	peripherals, err := mon.QomList("/machine/peripheral")
	if err != nil {
		return nil, err
	}

	netDeviceNames := make([]string, 0)

	for _, peripheral := range peripherals {
		if peripheral.Type == "child<virtio-net-pci>" {
			netDeviceNames = append(netDeviceNames, strings.TrimPrefix(peripheral.Name, "tap-"))
		}
	}

	return netDeviceNames, nil
}

func (d *driver) AttachNetworkAdapter(adapter NetworkAdapter) error {
	var device pcie.BusDevice
	switch opts := adapter.options.(type) {
	case tapNetworkAdapterOptions:
		device = pcie.NewTapNetworkDevice("tap-"+adapter.name, adapter.name, opts.macAddress)
	default:
		return errors.New("unsupported network adapter")
	}

	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	buses, err := mon.QueryPCI()
	if err != nil {
		return err
	}

	var emptyBridge *qmp.PciDevice

	for _, bus := range buses {
		for _, pciDevice := range bus.Devices {
			// we only care about root ports
			// (id 1b36:000c, see https://www.qemu.org/docs/master/specs/pci-ids.html)
			if pciDevice.Id.Vendor != 0x1b36 || pciDevice.Id.Device != 0x000c {
				continue
			}

			// cannot hotplug into used root port
			if len(pciDevice.PciBridge.Devices) > 0 {
				continue
			}

			emptyBridge = &pciDevice
		}
	}

	if emptyBridge == nil {
		return newRestartRequiredErr("no empty pci root port for hotplug")
	}

	hotplugs := device.GetHotplugs(pcie.BusAllocation{
		Bus:     emptyBridge.QdevId,
		Address: "00.0",
	})

	if len(hotplugs) != 1 {
		// network devices always return exactly one hotplug device
		panic("logic error")
	}

	err = hotplugs[0].Plug(mon)
	if err != nil {
		return fmt.Errorf("hotplugging network adapter: %w", err)
	}

	return nil
}

func (d *driver) DetachNetworkAdapter(name string) error {
	return newRestartRequiredErr("network adapter detach is not supported")
}

func (d *driver) GetVolumeIdentifiers() ([]DiskIdentifier, error) {
	mon, err := d.connectMonitor()
	if err != nil {
		return nil, err
	}

	qom, err := mon.QomList("/machine/peripheral")
	if err != nil {
		return nil, err
	}

	var paths []string

	for _, item := range qom {
		if item.Type == "child<scsi-hd>" || item.Type == "child<scsi-cd>" {
			paths = append(paths, "/machine/peripheral/"+item.Name)
		}
	}

	if len(paths) == 0 {
		return nil, nil
	}

	volumes, err := mon.QomListGet(paths)
	if err != nil {
		return nil, err
	}

	var volumeIds []DiskIdentifier

	for _, volume := range volumes {
		var vendor, product, serial string
		var ok bool

		for _, property := range volume.Properties {
			if property.Name == "vendor" {
				vendor, ok = property.Value.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected response from qemu, expected string, got %v", property.Value)
				}
			} else if property.Name == "product" {
				product, ok = property.Value.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected response from qemu, expected string, got %v", property.Value)
				}
			} else if property.Name == "serial" {
				serial, ok = property.Value.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected response from qemu, expected string, got %v", property.Value)
				}
			}
		}

		volumeIds = append(volumeIds, DiskIdFull(vendor, product, serial))
	}

	return volumeIds, nil
}

func (d *driver) AttachVolume(volume Volume) error {
	var device storage.ScsiDrive
	switch opts := volume.options.(type) {
	case cephVolumeOpts:
		device = storage.NewRbdDrive(volume.id.Serial, opts.pool, opts.name)
	default:
		return errors.New("unsupported volume type")
	}

	mon, err := d.connectMonitor()
	if err != nil {
		return err
	}

	hotplug := device.GetScsiHotplug("scsi.0") // TODO: feels bad to hardcode this
	err = hotplug.Plug(mon)
	if err != nil {
		return err
	}

	return nil
}

func (d *driver) DetachVolume(id DiskIdentifier) error {
	return newRestartRequiredErr("volume detach is not supported")
}
