package storage_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gwenya/qemu-driver/devices/storage"
)

var sampleIdentifiers = []struct {
	name   string
	testee storage.DiskIdentifier
}{
	{"empty identifier", storage.DiskIdentifier{}},
	{"short fields", storage.DiskIdentifier{Vendor: "a", Product: "b", Serial: "c"}},
	{"rootdisk", storage.DiskIdentifier{Vendor: "BEAN", Product: "STACK", Serial: "ROOTDISK"}},
	{"non-ascii serial", storage.DiskIdentifier{Vendor: "BEAN", Product: "STACK", Serial: "über-disk-é"}},
	{"serial longer than the node name", storage.DiskIdentifier{Vendor: "BEAN", Product: "STACK", Serial: strings.Repeat("x", 4096)}},
}

func TestDiskIdentifier_NodeName_KnownIdentifiersHashToFixedValues(t *testing.T) {
	// arrange
	testCases := []struct {
		name     string
		testee   storage.DiskIdentifier
		expected string
	}{
		{
			name:     "empty identifier",
			testee:   storage.DiskIdentifier{},
			expected: "nlqKW0iTyhcZ77pPDD4owkVfw2qNdxb",
		},
		{
			name:     "rootdisk, as the driver constructs it",
			testee:   storage.DiskIdentifier{Vendor: "BEAN", Product: "STACK", Serial: "ROOTDISK"},
			expected: "n-wu4W9KCulWOS7jJQrs3SlLyh1eJ65",
		},
		{
			name:     "cloudinit, as the driver constructs it",
			testee:   storage.DiskIdentifier{Vendor: "BEAN", Product: "STACK", Serial: "CLOUDINIT"},
			expected: "n4EaCjZZ6BwKiDXfiNyFTeTTvZLnP3N",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// act
			result := testCase.testee.NodeName()

			// assert
			assert.Equal(t, testCase.expected, result)
		})
	}
}

func TestDiskIdentifier_NodeName_IsAlways31Characters(t *testing.T) {
	for _, testCase := range sampleIdentifiers {
		t.Run(testCase.name, func(t *testing.T) {
			// act
			result := testCase.testee.NodeName()

			// assert
			assert.Len(t, result, 31)
		})
	}
}

func TestDiskIdentifier_NodeName_StartsWithLetterN(t *testing.T) {
	for _, testCase := range sampleIdentifiers {
		t.Run(testCase.name, func(t *testing.T) {
			// act
			result := testCase.testee.NodeName()

			// assert
			assert.Regexp(t, `^n`, result)
		})
	}
}

func TestDiskIdentifier_NodeName_UsesOnlyCharactersLegalInQemuNodeNames(t *testing.T) {
	for _, testCase := range sampleIdentifiers {
		t.Run(testCase.name, func(t *testing.T) {
			// act
			result := testCase.testee.NodeName()

			// assert
			assert.Regexp(t, `^[A-Za-z][A-Za-z0-9._-]*$`, result)
		})
	}
}

func TestDiskIdentifier_NodeName_DistinguishesFieldBoundaries(t *testing.T) {
	// arrange
	testee := storage.DiskIdentifier{Vendor: "a", Product: "b", Serial: "c"}
	shifted := storage.DiskIdentifier{Vendor: "ab", Product: "", Serial: "c"}

	// act
	result := testee.NodeName()
	shiftedResult := shifted.NodeName()

	// assert
	assert.NotEqual(t, result, shiftedResult)
}

func TestDiskIdentifier_NodeName_CollidesWhenAFieldContainsNul(t *testing.T) {
	// arrange
	testee := storage.DiskIdentifier{Vendor: "a\x00b", Product: "", Serial: "c"}
	colliding := storage.DiskIdentifier{Vendor: "a", Product: "b\x00", Serial: "c"}

	// act
	result := testee.NodeName()
	collidingResult := colliding.NodeName()

	// assert
	assert.Equal(t, result, collidingResult)
}
