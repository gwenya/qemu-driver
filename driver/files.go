package driver

type StorageFilename string
type RuntimeFilename string

const (
	RootDiskFileName    StorageFilename = "rootdisk.img"
	ConfigFileName      StorageFilename = "qemu.conf"
	QemuStdErrFileName  StorageFilename = "stderr.log"
	QemuStdOutFileName  StorageFilename = "stdout.log"
	FirmwareFileName    StorageFilename = "firmware.fd"
	NvramFileName       StorageFilename = "nvram.fd"
	CloudInitIsoFile    StorageFilename = "cloud-init.iso"
	CloudInitHashFile   StorageFilename = "cloud-init.sha256"
	CreatedFlagFileName StorageFilename = "created"
)

const (
	QemuQmpSocketFileName RuntimeFilename = "qmp.sock"
	ConsoleSocketFileName RuntimeFilename = "console.sock"
	QemuPidFileName       RuntimeFilename = "qemu.pid"
	QemuUnitNameFileName  RuntimeFilename = "qemu.unit-name"
)
