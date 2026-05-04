package main

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	"github.com/gwenya/qemu-driver/driver"
)

type logger struct{}

func (l *logger) Logf(format string, v ...interface{}) {
	fmt.Printf("[DRIVER] "+format+"\n", v...)
}

func main() {

	id := uuid.MustParse("804859f4-343b-4a0f-97b1-75d04aee531d")

	storagePath := path.Join("/tmp", id.String())

	imageSource := "/var/lib/beanstack/images/qcow2/remote/6d3a8507f767d6d28e84759bc530dc30997400283f425966d4808f1865dc1035/disk.qcow2"
	//firmwareSource := "/usr/share/edk2/ovmf/OVMF_CODE.fd"
	//nvramSource := "/usr/share/edk2/ovmf/OVMF_VARS.fd"

	err := os.MkdirAll(storagePath, os.ModePerm)
	if err != nil {
		panic(err)
	}

	//hwaddr := net.HardwareAddr{
	//	(byte(rand.Int31n(256)) & ^byte(0b1)) | byte(0b10),
	//	byte(rand.Int31n(256)),
	//	byte(rand.Int31n(256)),
	//	byte(rand.Int31n(256)),
	//	byte(rand.Int31n(256)),
	//	byte(rand.Int31n(256)),
	//}

	userData := `#cloud-config
users:
 - name: root
   lock_passwd: false
   hashed_passwd: "$6$rounds=4096$fBk6TX20Mwsv9IFd$HJHxFmF5xYZYSd2RwUP1ORduGJDmxSg/gotDjU6O5h19LebpRJuhsSr5mdmH84esQyJAxLbvlEIF7RHFyhbMb/"
   ssh_authorized_keys:
    - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB1DcFRiz4Z5fFiMfry6qcbe98GmpP4+Haj4KadbwCPz gwen@shark"

 - name: arch
   lock_passwd: false
   hashed_passwd: "$6$rounds=4096$UwlVflbzstVCE7Rc$3OezQvbACdvfKv.kP0SB/WRVc2kvpvd1DBmumGFxVVEMKYp/JdPpyIDLc/T3JYCFuXP8E8RA/tYIsV4LCPn6c."
   ssh_authorized_keys:
    - "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIB1DcFRiz4Z5fFiMfry6qcbe98GmpP4+Haj4KadbwCPz gwen@shark"
runcmd:
 - echo 'PermitRootLogin yes' >> /etc/ssh/sshd_config
`

	events := make(chan driver.Event)

	go func() {
		for {
			e := <-events
			fmt.Printf("EVENT: %s\n", e)
		}
	}()

	d, err := driver.New(
		driver.WithSystemId(id),
		driver.WithStorageDirectory(storagePath),
		driver.WithRuntimeDirectory(storagePath),
		driver.WithQemuPath("/usr/bin/qemu-system-x86_64"),
		driver.WithLogger(&logger{}),
		//driver.WithSystemdStrategy(driver.SystemdStrategyOptions{UnitNamePrefix: "qemu-"}, nil),
		driver.WithForkStrategy(nil),
		driver.WithEventChannel(events),
	)

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
		VsockCid:        5,
	})
	if err != nil {
		panic(err)
	}

	time.Sleep(time.Second * 1)

	err = d.Stop()
	if err != nil {
		panic(err)
	}

}
