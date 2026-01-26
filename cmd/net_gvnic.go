// Copyright (c) The go-boot authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build ignore

package cmd

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strings"

	"github.com/usbarmory/tamago/kvm/gvnic"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/tamago-example/shell"

	"github.com/gliderlabs/ssh"
	"github.com/usbarmory/go-net"

	"github.com/usbarmory/go-boot/uefi"
	"github.com/usbarmory/go-boot/uefi/x64"
)

const (
	VIRTIO_NET_PCI_VENDOR = 0x1ae0 // Google, Inc.
	VIRTIO_NET_PCI_DEVICE = 0x0042 // Compute Engine Virtual Ethernet [gVNIC]
)

func init() {
	shell.Add(shell.Cmd{
		Name: "net-gve",
		Help: "start gVNIC networking",
		Fn:   gvnicCmd,
	})
}

func gvnicCmd(console *shell.Interface, arg []string) (res string, err error) {
	gve := &gvnic.GVE{
		Device: pci.Probe(
			0,
			gvnic.PCI_VENDOR,
			gvnic.PCI_DEVICE,
		),
	}

	if err = gve.Init(); err != nil {
		return
	}

	return fmt.Sprintf("network initialized (%s)\n", gve.MAC()), nil
}
