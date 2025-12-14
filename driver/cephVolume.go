package driver

import "github.com/google/uuid"

type CephVolume struct {
	Id uuid.UUID
}

func (CephVolume) __volumeMarker() {}
