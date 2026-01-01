package chardev

import (
	"fmt"

	"github.com/gwenya/qemu-driver/config"
	"github.com/gwenya/qemu-driver/util"
)

type socketChardev struct {
	id   string
	opts SocketOpts
}

type SocketOpts struct {
	Unix        SocketOptsUnix
	Tcp         SocketOptsTcp
	Server      bool
	Wait        bool
	Telnet      bool
	Websocket   bool
	ReconnectMs int
	TlsCredsId  string
	TlsAuthzId  string
}

type SocketOptsTcp struct {
	Host    string
	Port    int
	To      int
	Ipv4    bool
	Ipv6    bool
	Nodelay bool
}

type SocketOptsUnix struct {
	Path string
}

func NewSocket(id string, opts SocketOpts) Chardev {
	return &socketChardev{
		id:   id,
		opts: opts,
	}
}

func (c *socketChardev) Config() config.Section {
	options := map[string]string{
		"server":    util.BoolToOnOff(c.opts.Server),
		"wait":      util.BoolToOnOff(c.opts.Wait),
		"telnet":    util.BoolToOnOff(c.opts.Telnet),
		"websocket": util.BoolToOnOff(c.opts.Websocket),
	}
	if c.opts.ReconnectMs != 0 {
		options["reconnect-ms"] = fmt.Sprintf("%d", c.opts.ReconnectMs)
	}

	if c.opts.TlsCredsId != "" {
		options["tls-creds"] = c.opts.TlsCredsId
	}

	if c.opts.TlsAuthzId != "" {
		options["tls-authz"] = c.opts.TlsAuthzId
	}

	if c.opts.Unix.Path != "" {
		options["path"] = c.opts.Unix.Path
	} else {
		if c.opts.Tcp.Host != "" {
			options["host"] = c.opts.Tcp.Host
		}

		if c.opts.Tcp.Port != 0 {
			options["port"] = fmt.Sprintf("%d", c.opts.Tcp.Port)
		}

		if c.opts.Tcp.To != 0 {
			options["to"] = fmt.Sprintf("%d", c.opts.Tcp.To)
		}

		options["nodelay"] = util.BoolToOnOff(c.opts.Tcp.Nodelay)

		if c.opts.Tcp.Ipv4 {
			options["ipv4"] = "on"
		}

		if c.opts.Tcp.Ipv6 {
			options["ipv6"] = "on"
		}

	}

	return chardevConfig(c.id, "socket", options)
}
