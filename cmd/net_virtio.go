// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"runtime/goos"
	"strings"

	"github.com/usbarmory/tamago/kvm/sev"
	"github.com/usbarmory/tamago/kvm/virtio"
	"github.com/usbarmory/tamago/soc/intel/ioapic"
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

	VIRTIO_NET_IRQ = 22
	// redirection vector for IOAPIC IRQ to CPU IRQ
	vector = 32
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

	if !sev.Features(x64.AMD64).SEV.SEV {
		x64.AllocateDMA(10 << 20)
	} else if err = initGHCB(); err != nil {
		return "", fmt.Errorf("could not initialize GHCB, %v", err)
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

	iface.Stack.EnableICMP()

	// hook interface into Go runtime
	net.SocketFunc = iface.Stack.Socket

	if x64.UEFI.Console.Out == 0 {
		// UEFI previosly terminated, use IRQs
		nic.Transport.EnableInterrupt(vector, vnet.ReceiveQueue)
		go startInterruptHandler(nic, iface)
	} else {
		// UEFI active, poll
		go iface.Start()
	}

	nic.Start()

	if len(arg[3]) > 0 {
		ip, _, _ := strings.Cut(arg[0], `/`)

		log.Printf("starting debug servers:\n")
		log.Printf("\thttp://%s:80/debug/pprof\n", ip)
		log.Printf("\tssh://%s:22\n", ip)

		ssh.Handle(sessionHandler)

		go ssh.ListenAndServe(":22", nil)
		go http.ListenAndServe(":80", nil)
	}

	mac, _ := iface.Stack.HardwareAddress()

	return fmt.Sprintf("network initialized (%s %s)\n", arg[0], mac), nil
}

func startInterruptHandler(dev *vnet.Net, iface *gnet.Interface) {
	if dev == nil || iface == nil {
		return
	}

	cpu := x64.AMD64

	if cpu.LAPIC != nil {
		cpu.LAPIC.Enable()
	}

	ioapic := &ioapic.IOAPIC{
		Base: 0xfec00000,
	}

	ioapic.EnableInterrupt(dev.IRQ, vector)

	// as IRQs are enabled, favor slicing dev.ReceiveWithHeader, opposed to
	// dev.Receive for better performance
	size := dev.HeaderLength + gnet.EthernetMaximumSize + gnet.MTU
	buf := make([]byte, size)

	isr := func(irq int) {
		switch irq {
		case vector:
			for {
				if n, err := dev.ReceiveWithHeader(buf); err != nil || n == 0 {
					return
				}

				iface.Stack.RecvInboundPacket(buf[dev.HeaderLength:])
			}
		default:
			log.Printf("internal error, unexpected IRQ %d", irq)
		}
	}

	// optimize CPU idle management as IRQs are enabled
	goos.Idle = func(pollUntil int64) {
		if pollUntil == 0 {
			return
		}

		cpu.SetAlarm(pollUntil)
		cpu.WaitInterrupt()
		cpu.SetAlarm(0)
	}

	cpu.ClearInterrupt()
	cpu.ServiceInterrupts(isr)
}
