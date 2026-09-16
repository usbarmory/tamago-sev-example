// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net"
	"os/signal"
	"regexp"
	"time"

	"github.com/usbarmory/tamago/kvm/virtio"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"

	"github.com/usbarmory/go-net"
	"github.com/usbarmory/go-net/virtio"

	"github.com/usbarmory/tamago-sev-example/internal/irq"
)

const (
	VIRTIO_NET_PCI_VENDOR = 0x1af4 // Red Hat, Inc.

	// Virtio 1.0 network device
	VIRTIO_NET_PCI_LEGACY_DEVICE = 0x1000
	VIRTIO_NET_PCI_MODERN_DEVICE = 0x1041

	// redirection vectors for IOAPIC IRQ to CPU IRQ or MSI-X signal
	VIRTIO_NET_IRQ = 32
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
	nic = &vnet.Net{
		IRQ:          VIRTIO_NET_IRQ,
		MTU:          gnet.MTU,
		HeaderLength: 10,
	}

	if device := pci.Probe(
		0,
		VIRTIO_NET_PCI_VENDOR,
		VIRTIO_NET_PCI_LEGACY_DEVICE,
	); device != nil {
		nic.Transport = &virtio.LegacyPCI{
			Device: device,
		}
	} else if device := pci.Probe(
		0,
		VIRTIO_NET_PCI_VENDOR,
		VIRTIO_NET_PCI_MODERN_DEVICE,
	); device != nil {
		nic.Transport = &virtio.PCI{
			Device: device,
		}
	}

	return
}

func virtioNetCmd(_ *shell.Interface, arg []string) (res string, err error) {
	nic := probeNIC()

	if nic == nil {
		return "", fmt.Errorf("could not find VirtIO network device")
	}

	if err := nic.Init(); err != nil {
		return "", fmt.Errorf("could not initialize VirtIO device, %v", err)
	}

	iface := &gnet.Interface{
		NetworkDevice: nic,
	}

	if arg[1] == ":" {
		arg[1] = ""
	}

	if err := iface.Init(arg[0], arg[1], arg[2]); err != nil {
		return "", fmt.Errorf("could not initialize networking, %v", err)
	}

	iface.HandleStackErr = func(err error, tx bool) {
		fmt.Printf("network stack error (tx:%v), %v", tx, err)
	}

	iface.Stack.EnableICMP()

	// hook interface into Go runtime
	net.SocketFunc = iface.Stack.Socket

	isr := func() {
		size := nic.HeaderLength + gnet.EthernetMaximumSize + gnet.MTU
		buf := make([]byte, size)

		// For better performance we slice dev.ReceiveWithHeader
		// instead of using dev.Receive.
		for {
			if n, err := nic.ReceiveWithHeader(buf); err != nil || n == 0 {
				return
			}

			iface.Stack.RecvInboundPacket(buf[nic.HeaderLength:])
		}
	}

	if err = nic.Transport.EnableInterrupt(nic.IRQ, vnet.ReceiveQueue); err != nil {
		return
	}

	irq.StartHandler(nic.IRQ, isr)

	// ensure ISR is running before starting the interface
	for !signal.Waiting() {
		time.Sleep(1 * time.Millisecond)
	}

	go nic.Start()

	mac, _ := iface.Stack.HardwareAddress()

	if len(arg[3]) > 0 {
		startDebugServices(arg[0], mac)
	}

	return fmt.Sprintf("network initialized (%s %s)\n", arg[0], mac), nil
}
