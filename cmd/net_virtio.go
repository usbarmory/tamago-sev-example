// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
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

	"github.com/usbarmory/tamago/kvm/sev"
	"github.com/usbarmory/tamago/kvm/virtio"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/usbarmory/go-net"
	"github.com/usbarmory/go-net/virtio"

	"github.com/gliderlabs/ssh"
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
		Syntax:  "<ip> <mac> <gw> (debug)?",
		Help:    "start VirtIO networking",
		Fn:      virtioNetCmd,
	})
}

func probeNIC() (nic *vnet.Net) {
	if device := pci.Probe(
		0,
		VIRTIO_NET_PCI_VENDOR,
		VIRTIO_NET_PCI_LEGACY_DEVICE,
	); device != nil {
		nic = &vnet.Net{
			Transport: &virtio.LegacyPCI{
				Device: device,
			},
		}
	} else if device := pci.Probe(
		0,
		VIRTIO_NET_PCI_VENDOR,
		VIRTIO_NET_PCI_MODERN_DEVICE,
	); device != nil {
		nic = &vnet.Net{
			Transport: &virtio.PCI{
				Device: device,
			},
		}
	}

	return
}

func virtioNetCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var nic *vnet.Net

	if nic = probeNIC(); nic == nil {
		return "", fmt.Errorf("could not find VirtIO network device")
	}

	nic.HeaderLength = 10
	nic.MTU = gnet.MTU

	if !sev.Features(x64.AMD64).SEV.SEV {
		x64.AllocateDMA(10 << 20)
	} else if err = initGHCB(); err != nil {
		return "", fmt.Errorf("could not initialize GHCB, %v", err)
	}

	if err := nic.Init(); err != nil {
		return "", fmt.Errorf("could not initialize VirtIO device, %v", err)
	}

	iface := gnet.Interface{
		NetworkDevice: nic,
	}

	if arg[1] == ":" {
		arg[1] = ""
	}

	if err := iface.Init(arg[0], arg[1], arg[2]); err != nil {
		return "", fmt.Errorf("could not initialize networking, %v", err)
	}

	if err = iface.Stack.EnableICMP(); err != nil {
		return "", fmt.Errorf("could not enable ICMP, %v", err)
	}

	// hook interface into Go runtime
	net.SocketFunc = iface.Stack.Socket

	nic.Start()
	go iface.Start()

	if len(arg[3]) > 0 {
		ip, _, _ := strings.Cut(arg[0], `/`)

		fmt.Printf("starting debug servers:\n")
		fmt.Printf("\thttp://%s:80/debug/pprof\n", ip)
		fmt.Printf("\tssh://%s:22\n", ip)

		ssh.Handle(sessionHandler)

		go ssh.ListenAndServe(":22", nil)
		go http.ListenAndServe(":80", nil)
	}

	mac, _ := iface.Stack.HardwareAddress()

	return fmt.Sprintf("network initialized (%s %s)\n", arg[0], mac), nil
}
