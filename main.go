package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"path"
	"slices"

	"github.com/gwenya/qemu-driver/driver"

	"github.com/google/uuid"
)

type logger struct{}

func (l *logger) Logf(format string, v ...interface{}) {
	fmt.Printf("[DRIVER] "+format+"\n", v...)
}

func main() {
	id := uuid.MustParse("804859f4-343b-4a0f-97b1-75d04aee531d")

	storagePath := path.Join("/tmp", id.String())

	imageSource := "/var/home/gwen/Downloads/Arch-Linux-x86_64-cloudimg-20251201.460866.qcow2"
	//firmwareSource := "/usr/share/edk2/ovmf/OVMF_CODE.fd"
	//nvramSource := "/usr/share/edk2/ovmf/OVMF_VARS.fd"

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

	userData := `#cloud-config
users:
  - name: root
    lock_passwd: false
    hashed_passwd: "$6$rounds=4096$nqxzsCUB62RiUjKp$YX0V8FDfz/9LdV6d5s0UGKBT8tAH2svzBILoS9Z/rWXjcny9Z9.ANt5XI6PU87268UrJrWeqtmH1lupgZtKZI/"

  - name: arch
    lock_passwd: false
    hashed_passwd: "$6$ekjadUze3yUXluSP$KTBA960c5FiFQIVHz7WQ8/9paorVjLWbnQ./NpUG8sE99ehX4SELqrMPEFq/yFKCB55i9gw6xbg.75i49WRlh/"

runcmd:
  - echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
`

	d, err := driver.New(driver.Options{
		SystemId:         id,
		StorageDirectory: storagePath,
		RuntimeDirectory: storagePath,
		QemuPath:         "/usr/bin/qemu-system-x86_64",
		Logger:           &logger{},
	})

	//	FirmwareSourcePath: firmwareSource,
	//	NvramSourcePath:    nvramSource,

	if err != nil {
		panic(err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer d.Close()

	if d.GetStatus() == driver.Uninitialized {
		err = d.Create(driver.CreateOptions{
			ImageSourcePath:   imageSource,
			ImageSourceFormat: "qcow2",
		})
		if err != nil {
			panic(err)
		}
	}

	if d.GetStatus() != driver.Running {
		err = d.Start(driver.StartOptions{
			CpuCount:   3,
			MemorySize: 1024 * 1024 * 1024,
			DiskSize:   10_000_000_000,
			CloudInit: driver.CloudInit{
				Meta: fmt.Sprintf("instance-id: %s", id),
				User: userData,
			},
			Volumes:         []driver.Volume{},
			NetworkAdapters: []driver.NetworkAdapter{},
			VsockCid:        3,
		})
		if err != nil {
			panic(err)
		}

	} else {
		names, err := d.GetNetworkAdapterNames()
		if err != nil {
			panic(err)
		}

		if slices.Contains(names, "test-tap") {
			err := d.DetachNetworkAdapter("test-tap")
			if err != nil {
				panic(err)
			}
		} else {
			err := d.AttachNetworkAdapter(driver.NewTapNetworkAdapter("test-tap", hwaddr))
			if err != nil {
				panic(err)
			}
		}

	}

}
