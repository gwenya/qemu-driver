package storage

import (
	"crypto/sha256"
	"encoding/base64"
)

type DiskIdentifier struct {
	Vendor  string
	Product string
	Serial  string
}

func (di *DiskIdentifier) NodeName() string {
	hasher := sha256.New()
	hasher.Write([]byte(di.Vendor))
	hasher.Write([]byte{0})
	hasher.Write([]byte(di.Product))
	hasher.Write([]byte{0})
	hasher.Write([]byte(di.Serial))

	hash := hasher.Sum(nil)

	hashStr := base64.RawURLEncoding.EncodeToString(hash)

	// the node name needs to start with an alphabetic character but base64 contains digits and special chars
	// so we just prefix it with an "n" (for "node")
	// max length for node names is 31
	return "n" + hashStr[:30]
}
