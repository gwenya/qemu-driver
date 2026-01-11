package driver

import (
	"github.com/gwenya/qemu-driver/devices/storage"
)

type DiskIdentifier struct {
	Vendor  string
	Product string
	Serial  string
}

func (i DiskIdentifier) toStorage() storage.DiskIdentifier {
	return storage.DiskIdentifier{
		Vendor:  i.Vendor,
		Product: i.Product,
		Serial:  i.Serial,
	}
}

func DiskId(serial string) DiskIdentifier {
	return DiskIdentifier{
		Vendor:  "BEAN",
		Product: "STACK",
		Serial:  serial,
	}
}

func diskIdRootdisk() DiskIdentifier {
	return DiskIdFull("BEAN", "STACK", "ROOTDISK")
}

func diskIdCloudInit() DiskIdentifier {
	return DiskIdFull("BEAN", "STACK", "CLOUDINIT")
}

func DiskIdFull(vendor string, model string, serial string) DiskIdentifier {
	return DiskIdentifier{
		Vendor:  vendor,
		Product: model,
		Serial:  serial,
	}
}
