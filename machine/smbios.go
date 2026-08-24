package machine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type smbiosEntry struct {
	smbiosType int
	fields     map[string]string
}

type BiosInfo struct {
	Vendor  string
	Version string
	Date    string
	Release string
	Uefi    *bool
}

type SystemInfo struct {
	Manufacturer string
	Product      string
	Version      string
	Serial       string
	Uuid         string
	Sku          string
	Family       string
}

type BaseboardInfo struct {
	Manufacturer string
	Product      string
	Version      string
	Serial       string
	Asset        string
	Location     string
}

type ChassisInfo struct {
	Manufacturer string
	Version      string
	Serial       string
	Asset        string
	Sku          string
}

type ProcessorInfo struct {
	SockPfx         string
	Manufacturer    string
	Version         string
	Serial          string
	Asset           string
	Part            string
	ProcessorFamily *int
	ProcessorId     *int
}

type MemoryDeviceInfo struct {
	LocPfx       string
	Bank         string
	Manufacturer string
	Serial       string
	Asset        string
	Part         string
	Speed        *int
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (d *Description) addSmbios(smbiosType int, fields map[string]string) {
	d.smbiosEntries = append(d.smbiosEntries, smbiosEntry{smbiosType: smbiosType, fields: fields})
}

func (d *Description) AddRawSmbios(smbiosType int, fields map[string]string) {
	d.addSmbios(smbiosType, fields)
}

func (d *Description) AddBiosInfo(info BiosInfo) {
	fields := map[string]string{
		"vendor":  info.Vendor,
		"version": info.Version,
		"date":    info.Date,
		"release": info.Release,
	}
	if info.Uefi != nil {
		fields["uefi"] = onOff(*info.Uefi)
	}
	d.addSmbios(0, fields)
}

func (d *Description) AddSystemInfo(info SystemInfo) {
	d.addSmbios(1, map[string]string{
		"manufacturer": info.Manufacturer,
		"product":      info.Product,
		"version":      info.Version,
		"serial":       info.Serial,
		"uuid":         info.Uuid,
		"sku":          info.Sku,
		"family":       info.Family,
	})
}

func (d *Description) AddBaseboardInfo(info BaseboardInfo) {
	d.addSmbios(2, map[string]string{
		"manufacturer": info.Manufacturer,
		"product":      info.Product,
		"version":      info.Version,
		"serial":       info.Serial,
		"asset":        info.Asset,
		"location":     info.Location,
	})
}

func (d *Description) AddChassisInfo(info ChassisInfo) {
	d.addSmbios(3, map[string]string{
		"manufacturer": info.Manufacturer,
		"version":      info.Version,
		"serial":       info.Serial,
		"asset":        info.Asset,
		"sku":          info.Sku,
	})
}

func (d *Description) AddProcessorInfo(info ProcessorInfo) {
	fields := map[string]string{
		"sock_pfx":     info.SockPfx,
		"manufacturer": info.Manufacturer,
		"version":      info.Version,
		"serial":       info.Serial,
		"asset":        info.Asset,
		"part":         info.Part,
	}
	if info.ProcessorFamily != nil {
		fields["processor-family"] = strconv.Itoa(*info.ProcessorFamily)
	}
	if info.ProcessorId != nil {
		fields["processor-id"] = strconv.Itoa(*info.ProcessorId)
	}
	d.addSmbios(4, fields)
}

func (d *Description) AddOemString(value string) {
	d.addSmbios(11, map[string]string{"value": value})
}

func (d *Description) AddMemoryDeviceInfo(info MemoryDeviceInfo) {
	fields := map[string]string{
		"loc_pfx":      info.LocPfx,
		"bank":         info.Bank,
		"manufacturer": info.Manufacturer,
		"serial":       info.Serial,
		"asset":        info.Asset,
		"part":         info.Part,
	}
	if info.Speed != nil {
		fields["speed"] = strconv.Itoa(*info.Speed)
	}
	d.addSmbios(17, fields)
}

func (d *Description) SmbiosArgs() []string {
	var args []string

	for _, entry := range d.smbiosEntries {
		parts := []string{fmt.Sprintf("type=%d", entry.smbiosType)}

		keys := make([]string, 0, len(entry.fields))
		for k, v := range entry.fields {
			if v != "" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)

		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", k, entry.fields[k]))
		}

		args = append(args, "-smbios", strings.Join(parts, ","))
	}

	return args
}
