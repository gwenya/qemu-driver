package driver

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/kdomanski/iso9660"
)

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
	VsockCid           uint32
	CloudInit          CloudInitData
}

type CloudInitData struct {
	Vendor  string
	Meta    string
	User    string
	Network string
}

func (d *CloudInitData) CreateIso(path string) error {
	writer, err := iso9660.NewWriter()
	if err != nil {
		return fmt.Errorf("creating iso writer: %w", err)
	}

	defer writer.Cleanup()

	err = writer.AddFile(strings.NewReader(d.User), "user-data")
	if err != nil {
		return fmt.Errorf("adding user-data: %w", err)
	}

	err = writer.AddFile(strings.NewReader(d.Meta), "meta-data")
	if err != nil {
		return fmt.Errorf("adding meta-data: %w", err)
	}

	if d.Vendor != "" {
		err = writer.AddFile(strings.NewReader(d.Vendor), "vendor-data")
		if err != nil {
			return fmt.Errorf("adding vendor-data: %w", err)
		}
	}

	if d.Network != "" {
		err = writer.AddFile(strings.NewReader(d.Network), "network-config")
		if err != nil {
			return fmt.Errorf("adding network-config: %w", err)
		}
	}

	isoFile, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("failed to create file: %s", err)
	}
	defer isoFile.Close()

	err = writer.WriteTo(isoFile, "cidata")
	if err != nil {
		return fmt.Errorf("writing iso file: %w", err)
	}

	return nil
}
