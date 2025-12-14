package driver

import "github.com/google/uuid"

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
