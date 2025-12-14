package driver

import (
	"net"

	"github.com/google/uuid"
)

type NetworkInterface interface {
	__networkInterfaceMarker()
}

type NetworkInterfaceBase struct {
	Id         uuid.UUID
	MacAddress net.HardwareAddr
}
