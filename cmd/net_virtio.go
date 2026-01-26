// Copyright (c) The go-boot authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strings"

	"github.com/usbarmory/tamago/soc/intel/pci"
	"github.com/usbarmory/tamago/kvm/sev"
	"github.com/usbarmory/tamago/kvm/virtio"

	"github.com/usbarmory/tamago-example/shell"

	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/gliderlabs/ssh"
	"github.com/usbarmory/virtio-net"
)

const (
	VIRTIO_NET_PCI_VENDOR = 0x1af4 // Red Hat, Inc.

	// Virtio 1.0 network device
	VIRTIO_NET_PCI_LEGACY_DEVICE = 0x1000
	VIRTIO_NET_PCI_MODERN_DEVICE = 0x1041
)

func init() {
	shell.Add(shell.Cmd{
		Name:    "net-virtio",
		Args:    4,
		Pattern: regexp.MustCompile(`^net-virtio (\S+) (\S+) (\S+)( debug)?$`),
		Syntax:  "<ip> <mask> <gw> (debug)?",
		Help:    "start VirtIO networking",
		Fn:      virtioNetCmd,
	})
}

func virtioNetCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var nic *vnet.Net
	iface := vnet.Interface{}

	if !sev.Features(x64.AMD64).SEV.SEV {
		x64.AllocateDMA(2 << 20)
	} else if err = initGHCB(); err != nil {
		return "", fmt.Errorf("could not initialize GHCB, %v", err)
	}

	if device := pci.Probe(
		0,
		VIRTIO_NET_PCI_VENDOR,
		VIRTIO_NET_PCI_LEGACY_DEVICE,
	); device != nil {
		nic = &vnet.Net{
			Transport:    &virtio.LegacyPCI{
				Device: device,
			},
			HeaderLength: 10,
		}
	} else if device := pci.Probe(
		0,
		VIRTIO_NET_PCI_VENDOR,
		VIRTIO_NET_PCI_MODERN_DEVICE,
	); device != nil {
		nic = &vnet.Net{
			Transport:    &virtio.PCI{
				Device: device,
			},
			HeaderLength: 10,
		}
	}

	if nic == nil {
		return "", fmt.Errorf("could not find VirtIO network device")
	}

	if err := iface.Init(nic, arg[0], arg[1], arg[2]); err != nil {
		return "", fmt.Errorf("could not initialize networking, %v", err)
	}

	iface.EnableICMP()
	go nic.Start(true) // TODO: IRQs

	// hook interface into Go runtime
	net.SocketFunc = iface.Socket

	if len(arg[3]) > 0 {
		ip, _, _ := strings.Cut(arg[0], `/`)

		fmt.Printf("starting debug servers:\n")
		fmt.Printf("\thttp://%s:80/debug/pprof\n", ip)
		fmt.Printf("\tssh://%s:22\n", ip)

		ssh.Handle(func(s ssh.Session) {
			c := &shell.Interface{
				Banner:     Banner,
				ReadWriter: s,
			}
			c.Start(true)
		})

		go ssh.ListenAndServe(":22", nil)
		go http.ListenAndServe(":80", nil)
	}

	return fmt.Sprintf("network initialized (%s %x)\n", arg[0], nic.Config().MAC), nil
}
