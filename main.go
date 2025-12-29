package main

import (
	"math/rand"
	"net"
	"os"
	"path"

	"github.com/gwenya/qemu-driver/driver"

	"github.com/google/uuid"
)

func main() {
	id := uuid.MustParse("804859f4-343b-4a0f-97b1-75d04aee531d")

	storagePath := path.Join("/tmp", id.String())

	imageSource := "/var/home/gwen/Downloads/Arch-Linux-x86_64-basic.qcow2"
	firmwareSource := "/usr/share/edk2/ovmf/OVMF_CODE.fd"
	nvramSource := "/usr/share/edk2/ovmf/OVMF_VARS.fd"

	err := os.MkdirAll(storagePath, os.ModePerm)
	if err != nil {
		panic(err)
	}

	hwaddr := net.HardwareAddr{
		(byte(rand.Int31n(256)) & ^byte(0b1)) | byte(0b10),
		byte(rand.Int31n(256)),
		byte(rand.Int31n(256)),
		byte(rand.Int31n(256)),
		byte(rand.Int31n(256)),
		byte(rand.Int31n(256)),
	}

	d, err := driver.New("/usr/bin/qemu-system-x86_64", driver.MachineConfiguration{
		Id:                 id,
		StoragePath:        storagePath,
		ImageSourcePath:    imageSource,
		FirmwareSourcePath: firmwareSource,
		NvramSourcePath:    nvramSource,
		CpuCount:           1,
		MemorySize:         1024 * 1024 * 1024,
		DiskSize:           50_000_000_000,
		NetworkInterfaces: []driver.NetworkInterface{
			driver.NewTapNetworkInterface("test-tap", hwaddr),
		},
		Volumes: nil,
	})

	if err != nil {
		panic(err)
	}

	err = d.Create()
	if err != nil {
		panic(err)
	}

	err = d.Start()
	if err != nil {
		panic(err)
	}
}
