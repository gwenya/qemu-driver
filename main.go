package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path"

	"github.com/gwenya/qemu-driver/driver"

	"github.com/google/uuid"
)

func main() {
	id := uuid.MustParse("804859f4-343b-4a0f-97b1-75d04aee531d")

	storagePath := path.Join("/tmp", id.String())

	imageSource := "/var/home/gwen/Downloads/Arch-Linux-x86_64-cloudimg-20251201.460866.qcow2"
	firmwareSource := "/usr/share/edk2/ovmf/OVMF_CODE.fd"
	nvramSource := "/usr/share/edk2/ovmf/OVMF_VARS.fd"

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

	d, err := driver.New("/usr/bin/qemu-system-x86_64", driver.MachineConfiguration{
		Id:                 id,
		StoragePath:        storagePath,
		ImageSourcePath:    imageSource,
		FirmwareSourcePath: firmwareSource,
		NvramSourcePath:    nvramSource,
		CpuCount:           1,
		MemorySize:         1024 * 1024 * 1024,
		DiskSize:           50_000_000_000,
		NetworkInterfaces:  []driver.NetworkInterface{
			//driver.NewTapNetworkInterface("test-tap", hwaddr),
		},
		Volumes:  nil,
		VsockCid: 3,
		CloudInit: driver.CloudInitData{
			Vendor:  base64.RawStdEncoding.EncodeToString([]byte(fmt.Sprintf("instance-id: %s", id))),
			Meta:    "",
			User:    "I2Nsb3VkLWNvbmZpZwp1c2VyczoKICAtIG5hbWU6IHJvb3QKICAgIGxvY2tfcGFzc3dkOiBmYWxzZQogICAgaGFzaGVkX3Bhc3N3ZDogIiQ2JHJvdW5kcz00MDk2JG5xeHpzQ1VCNjJSaVVqS3AkWVgwVjhGRGZ6LzlMZFY2ZDVzMFVHS0JUOHRBSDJzdnpCSUxvUzlaL3JXWGpjbnk5WjkuQU50NVhJNlBVODcyNjhVckpyV2VxdG1IMWx1cGdadEtaSS8iCgogIC0gbmFtZTogYXJjaAogICAgbG9ja19wYXNzd2Q6IGZhbHNlCiAgICBoYXNoZWRfcGFzc3dkOiAiJDYkZWtqYWRVemUzeVVYbHVTUCRLVEJBOTYwYzVGaUZRSVZIejdXUTgvOXBhb3JWakxXYm5RLi9OcFVHOHNFOTllaFg0U0VMcXJNUEVGcS95RktDQjU1aTlndzZ4YmcuNzVpNDlXUmxoLyIKCnJ1bmNtZDoKICAtIGVjaG8gJ1Blcm1pdFJvb3RMb2dpbiB5ZXMnID4",
			Network: "",
		},
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
