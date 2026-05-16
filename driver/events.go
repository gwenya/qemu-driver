package driver

type Event interface {
	__isEvent()
}

type StartedEvent struct{}

type StoppedEvent struct {
	Guest bool
}

type ProcessExitEvent struct{}

type RestartedEvent struct {
	Guest bool
}

func (StartedEvent) String() string {
	return "Started"
}

func (StartedEvent) __isEvent() {}

func (e StoppedEvent) String() string {
	if e.Guest {
		return "Stopped by guest"
	}

	return "Stopped by hypervisor"
}

func (StoppedEvent) __isEvent() {}

func (ProcessExitEvent) __isEvent() {}

func (e RestartedEvent) String() string {
	if e.Guest {
		return "Restarted by guest"
	}

	return "Restarted by hypervisor"
}

func (RestartedEvent) __isEvent() {}
