package systemd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

type Waiter interface {
	Add(unitName string) (chan struct{}, error)
	Remove(unitName string) error

	io.Closer
}

type waiter struct {
	conn           *dbus.Conn
	unitNamePrefix string
	unitMap        map[string]chan struct{}
	mu             sync.Mutex
	runDone        chan struct{}
	cancel         context.CancelFunc
}

func NewWaiter(unitNamePrefix string) (Waiter, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := &waiter{
		conn:           conn,
		unitNamePrefix: unitNamePrefix,
		unitMap:        make(map[string]chan struct{}),
		runDone:        make(chan struct{}),
		cancel:         cancel,
	}

	err = w.conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.systemd1"),
		dbus.WithMatchArg(0, "org.freedesktop.systemd1.Unit"),
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
	)
	if err != nil {
		return nil, fmt.Errorf("setting up systemd signal matching: %w", err)
	}

	dbusObj := w.conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1")
	err = dbusObj.Call("org.freedesktop.systemd1.Manager.Subscribe", 0).Err
	if err != nil {
		return nil, fmt.Errorf("subscribing to systemd signals: %w", err)
	}

	go w.run(ctx)

	return w, nil
}

func (w *waiter) run(ctx context.Context) {
	ch := make(chan *dbus.Signal)
	w.conn.Signal(ch)

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case sig := <-ch:
			go w.handleSignal(ctx, sig)
		}
	}

	w.conn.RemoveSignal(ch)
	close(w.runDone)
}

const unitPathPrefix = "/org/freedesktop/systemd1/unit/"

func (w *waiter) handleSignal(ctx context.Context, signal *dbus.Signal) {
	if !strings.HasPrefix(string(signal.Path), unitPathPrefix) {
		return
	}

	unitNameEscaped := strings.TrimPrefix(string(signal.Path), unitPathPrefix)
	unitName := PathBusUnescape(unitNameEscaped)

	if !strings.HasPrefix(unitName, w.unitNamePrefix) {
		return
	}

	properties, ok := signal.Body[1].(map[string]dbus.Variant)
	if !ok {
		fmt.Printf("invalid signal body: %v\n", signal.Body) // TODO: use logger
		return
	}

	if prop, ok := properties["InactiveEnterTimestamp"]; !ok {
		return
	} else if val, ok := prop.Value().(uint64); !ok {
		return
	} else if val == 0 {
		return
	}

	if prop, ok := properties["ActiveState"]; !ok {
		return
	} else if val, ok := prop.Value().(string); !ok {
		return
	} else if val == "active" {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	ch, ok := w.unitMap[unitName]
	close(ch)
}

func (w *waiter) Add(unitName string) (chan struct{}, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.unitMap[unitName]; exists {
		return nil, fmt.Errorf("already waiting for unit %s", unitName)
	}

	ch := make(chan struct{})
	w.unitMap[unitName] = ch
	return ch, nil
}

func (w *waiter) Remove(unitName string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	_, exists := w.unitMap[unitName]
	if !exists {
		return fmt.Errorf("unit %s is not being waited on", unitName)
	}

	delete(w.unitMap, unitName)
	return nil
}

func (w *waiter) Close() error {
	w.cancel()
	<-w.runDone
	return nil
}
