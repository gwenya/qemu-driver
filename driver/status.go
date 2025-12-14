package driver

type Status string

const (
	Uninitialized Status = "uninitialized"
	Starting      Status = "starting"
	Stopping      Status = "stopping"
	Restarting    Status = "restarting"
	Running       Status = "running"
	Stopped       Status = "stopped"
	Unknown       Status = "unknown"
)
