package qmp

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/digitalocean/go-qemu/qmp"
)

type Monitor interface {
	AddDevice(device map[string]any) error
	AddBlockDevice(blockDev map[string]any) error
	Continue() error
	Quit() error

	Disconnect() error
}

type monitor struct {
	q qmp.Monitor
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

func (m *monitor) runCommand(command string, args map[string]any) error {
	cmd := map[string]any{
		"execute": command,
	}

	if args != nil {
		cmd["arguments"] = args
	}

	jsonBytes, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	_, err = m.q.Run(jsonBytes)
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
