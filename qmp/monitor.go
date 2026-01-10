package qmp

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
)

type Monitor interface {
	AddDevice(device map[string]any) error
	AddBlockDevice(blockDev map[string]any) error
	AddNetworkDevice(netDev map[string]any) error
	Continue() error
	Quit() error
	Disconnect() error
	Status() (RunState, error)
	QueryCPUs() ([]CpuInfo, error)
	QueryHotpluggableCPUs() ([]HotpluggableCpu, error)
	QueryMemorySummary() (MemorySummary, error)
	QueryMemoryDevices() ([]MemoryDevice, error)
	QueryPCI() ([]PciBus, error)
	QomList(path string) ([]QomInfo, error)
	QomListGet(paths []string) ([]QomProperties, error)
	AddMemoryBackend(id string, size uint64) error
	RemoveMemoryBackend(id string) error
	SendFd(name string, fd *os.File) error
	CloseFd(name string) error
}

type monitor struct {
	q *qmp.SocketMonitor
}

func Connect(qmpSocketPath string) (Monitor, error) {
	m, err := qmp.NewSocketMonitor("unix", qmpSocketPath, time.Second*10)
	if err != nil {
		return nil, fmt.Errorf("creating socket monitor: %w", err)
	}

	err = m.Connect()
	if err != nil {
		return nil, fmt.Errorf("connecting socket monitor: %w", err)
	}

	// TODO: event listener

	return &monitor{
		q: m,
	}, nil
}

func serializeCommand(command string, args map[string]any) ([]byte, error) {
	cmd := map[string]any{
		"execute": command,
	}

	if args != nil {
		cmd["arguments"] = args
	}

	return json.Marshal(cmd)
}

func (m *monitor) runCommand(command string, args map[string]any) error {
	cmd, err := serializeCommand(command, args)
	if err != nil {
		return err
	}

	fmt.Printf("running command %s\n", string(cmd))

	_, err = m.q.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (m *monitor) runCommandWithFd(command string, args map[string]any, fd *os.File) error {
	cmd, err := serializeCommand(command, args)
	if err != nil {
		return err
	}

	_, err = m.q.RunWithFile(cmd, fd)
	if err != nil {
		return err
	}

	return nil
}

func (m *monitor) runCommandsWithResponse(command string, args map[string]any, resp any) error {
	cmd, err := serializeCommand(command, args)
	if err != nil {
		return err
	}

	respBytes, err := m.q.Run(cmd)
	if err != nil {
		return err
	}

	if resp == nil {
		return nil
	}

	err = json.Unmarshal(respBytes, resp)
	if err != nil {
		return err
	}

	return nil
}

func (m *monitor) AddDevice(device map[string]any) error {
	return m.runCommand("device_add", device)
}

func (m *monitor) AddBlockDevice(blockDev map[string]any) error {
	return m.runCommand("blockdev-add", blockDev)
}

func (m *monitor) AddNetworkDevice(netDev map[string]any) error {
	return m.runCommand("netdev_add", netDev)
}

func (m *monitor) Continue() error {
	return m.runCommand("cont", nil)
}

func (m *monitor) Quit() error {
	return m.runCommand("quit", nil)
}

func (m *monitor) Disconnect() error {
	err := m.q.Disconnect()
	if err != nil {
		return fmt.Errorf("disconnecting qmp: %w", err)
	}

	return nil
}

func (m *monitor) Status() (RunState, error) {
	var resp Response[struct {
		Status RunState `json:"status"`
	}]
	err := m.runCommandsWithResponse("query-status", nil, &resp)
	if err != nil {
		return "", err
	}

	return resp.Return.Status, nil
}

func (m *monitor) QueryCPUs() ([]CpuInfo, error) {
	var resp Response[[]CpuInfo]
	err := m.runCommandsWithResponse("query-cpus-fast", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Return, nil
}

func (m *monitor) QueryHotpluggableCPUs() ([]HotpluggableCpu, error) {
	var resp Response[[]HotpluggableCpu]
	err := m.runCommandsWithResponse("query-hotpluggable-cpus", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Return, nil
}

func (m *monitor) QueryMemorySummary() (MemorySummary, error) {
	var resp Response[MemorySummary]
	err := m.runCommandsWithResponse("query-memory-size-summary", nil, &resp)
	if err != nil {
		return MemorySummary{}, err
	}

	return resp.Return, nil
}

func (m *monitor) QueryMemoryDevices() ([]MemoryDevice, error) {
	var resp Response[[]MemoryDevice]
	err := m.runCommandsWithResponse("query-memory-devices", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Return, nil
}

func (m *monitor) AddMemoryBackend(id string, size uint64) error {
	err := m.runCommand("object-add", map[string]any{
		"id":       id,
		"qom-type": "memory-backend-ram",
		"size":     size,
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *monitor) RemoveMemoryBackend(id string) error {
	err := m.runCommand("object-del", map[string]any{
		"id": id,
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *monitor) QueryPCI() ([]PciBus, error) {
	var resp Response[[]PciBus]
	err := m.runCommandsWithResponse("query-pci", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Return, nil
}

type QomInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QomProperties struct {
	Properties []QomValue `json:"properties"`
}

type QomValue struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

func (m *monitor) QomList(path string) ([]QomInfo, error) {
	var resp Response[[]QomInfo]
	err := m.runCommandsWithResponse("qom-list", map[string]any{"path": path}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Return, nil
}

func (m *monitor) QomListGet(paths []string) ([]QomProperties, error) {
	var resp Response[[]QomProperties]
	err := m.runCommandsWithResponse("qom-list-get", map[string]any{"paths": paths}, &resp)
	if err != nil {
		return nil, err
	}

	return resp.Return, nil
}

func (m *monitor) SendFd(name string, fd *os.File) error {
	err := m.runCommandWithFd("getfd", map[string]any{
		"fdname": name,
	}, fd)

	if err != nil {
		return err
	}

	return nil
}

func (m *monitor) CloseFd(name string) error {
	err := m.runCommand("closefd", map[string]any{
		"fdname": name,
	})

	if err != nil {
		return err
	}

	return nil
}
