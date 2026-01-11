package pcie

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/devices"
	"github.com/gwenya/qemu-driver/qmp"
	"golang.org/x/sys/unix"
)

type tapNetworkDevice struct {
	hotPlug
	id         string
	netdevName string
	hwaddr     net.HardwareAddr
}

func NewTapNetworkDevice(id string, netdevName string, hwaddr net.HardwareAddr) BusDevice {
	return &tapNetworkDevice{
		id:         id,
		netdevName: netdevName,
		hwaddr:     hwaddr,
	}
}

func (d *tapNetworkDevice) Config(_ BusAllocation) []config.Section {
	return nil
}

func (d *tapNetworkDevice) GetHotplugs(alloc BusAllocation) []devices.HotplugDevice {
	return []devices.HotplugDevice{
		hotplugWrap(d, alloc),
	}
}

func (d *tapNetworkDevice) Plug(m qmp.Monitor, alloc BusAllocation) (errRet error) {
	// TODO: only add the netdev if it doesn't exist yet, and only add the device if it doesn't exist yet
	// just ignoring the error from netdev add is not enough since at that point the fds for the tap are already allocated and sent to qemu
	cpus, err := m.QueryCPUs()
	if err != nil {
		return fmt.Errorf("getting cpu info from qemu: %w", err)
	}

	queueCount := len(cpus)

	fds := make([]*os.File, 0, queueCount)
	fdNames := make([]string, 0, queueCount)
	vhostFds := make([]*os.File, 0, queueCount)
	vhostFdNames := make([]string, 0, queueCount)

	defer func() {
		for _, fd := range fds {
			_ = fd.Close()
		}

		for _, fd := range vhostFds {
			_ = fd.Close()
		}

		if errRet != nil {
			for _, name := range fdNames {
				_ = m.CloseFd(name)
			}
		}
	}()

	for i := range queueCount {
		fd, err := openTap(d.netdevName)
		if err != nil {
			return fmt.Errorf("opening tap for queue %d: %w", i, err)
		}

		fds = append(fds, fd)

		tapFdName := fmt.Sprintf("%s.%d", d.netdevName, i)

		err = m.SendFd(tapFdName, fd)
		if err != nil {
			return fmt.Errorf("sending fd %s to qemu: %w", tapFdName, err)
		}

		fdNames = append(fdNames, tapFdName)

		vhostFd, err := os.OpenFile("/dev/vhost-net", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("opening /dev/vhost-net: %w", err)
		}

		vhostFds = append(vhostFds, vhostFd)

		vhostFdName := fmt.Sprintf("%s.vhost.%d", d.netdevName, i)

		err = m.SendFd(vhostFdName, vhostFd)
		if err != nil {
			return fmt.Errorf("sending fd %s to qemu: %w", tapFdName, err)
		}

		vhostFdNames = append(vhostFdNames, vhostFdName)
	}

	netdevId := d.id + "-netdev"

	numVectors := queueCount*2 + 2

	err = m.AddNetworkDevice(map[string]any{
		"id":       netdevId,
		"type":     "tap",
		"vhost":    true,
		"fds":      strings.Join(fdNames, ":"),
		"vhostfds": strings.Join(vhostFdNames, ":"),
	})
	if err != nil {
		return fmt.Errorf("adding netdev to qemu: %w", err)
	}

	err = m.AddDevice(map[string]any{
		"id":            d.id,
		"driver":        "virtio-net-pci",
		"netdev":        netdevId,
		"mac":           d.hwaddr.String(),
		"mq":            true,
		"vectors":       numVectors,
		"bus":           alloc.Bus,
		"addr":          alloc.Address,
		"multifunction": alloc.Multifunction,
	})
	if err != nil {
		return fmt.Errorf("adding device to qemu: %w", err)
	}

	return nil
}

func openTap(name string) (f *os.File, retErr error) {
	fd, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening /dev/net/tun: %w", err)
	}

	defer func() {
		if retErr != nil {
			_ = fd.Close()
		}
	}()

	ifr, err := unix.NewIfreq(name)
	if err != nil {
		return nil, fmt.Errorf("creating ifreq for %s: %w", name, err)
	}

	ifr.SetUint16(unix.IFF_TAP | unix.IFF_MULTI_QUEUE | unix.IFF_NO_PI | unix.IFF_VNET_HDR)

	err = unix.IoctlIfreq(int(fd.Fd()), unix.TUNSETIFF, ifr)
	if err != nil {
		return nil, fmt.Errorf("configuring tap on %s: %w", name, err)
	}

	return fd, nil
}

func (d *tapNetworkDevice) Unplug(m qmp.Monitor, alloc BusAllocation) error {
	//TODO implement me
	panic("implement me")
}

func UnplugTapNetworkDevice(m qmp.Monitor, name string) error {
	err := m.DeleteDevice("tap-" + name)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}

	err = m.DeleteNetworkDevice("tap-" + name + "-netdev")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return err
	}

	return nil
}
