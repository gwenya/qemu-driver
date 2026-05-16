module github.com/gwenya/qemu-driver

go 1.25.4

require (
	codeberg.org/gwenya/go-fanout v0.1.0
	github.com/coreos/go-systemd/v22 v22.7.0
	github.com/digitalocean/go-qemu v0.0.0-20250212194115-ee9b0668d242
	github.com/godbus/dbus/v5 v5.2.2
	github.com/google/uuid v1.6.0
	github.com/kdomanski/iso9660 v0.4.0
	github.com/urfave/cli/v3 v3.9.0
	golang.org/x/sys v0.44.0
)

require github.com/digitalocean/go-libvirt v0.0.0-20220804181439-8648fbde413e // indirect
