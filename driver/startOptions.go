package driver

import "github.com/gwenya/qemu-driver/machine"

type StartOptions struct {
	CpuCount        uint32
	MemorySize      uint64
	DiskSize        uint64
	ReadonlyDisk    bool
	CloudInit       CloudInit
	Volumes         []Volume
	NetworkAdapters []NetworkAdapter
	VsockCid        uint32
	SystemInfo      *machine.SystemInfo
	ChassisInfo     *machine.ChassisInfo
	OemStrings      []string
}
