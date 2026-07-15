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
	"os/signal"
	"regexp"
	"runtime/goos"
	"strings"
	"time"

	"github.com/usbarmory/tamago/kvm/virtio"
	"github.com/usbarmory/tamago/soc/intel/ioapic"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/usbarmory/go-net"
	"github.com/usbarmory/go-net/virtio"

	"github.com/usbarmory/tamago-sev-example/internal/ssh"
)

const (
	IOAPIC0_BASE = 0xfec00000

	VIRTIO_NET_PCI_VENDOR = 0x1af4 // Red Hat, Inc.

	// Virtio 1.0 network device
	VIRTIO_NET_PCI_LEGACY_DEVICE = 0x1000
	VIRTIO_NET_PCI_MODERN_DEVICE = 0x1041

	// redirection vector for IOAPIC IRQ to CPU IRQ or MSI-X signal
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

	iface.Stack.EnableICMP()

	// hook interface into Go runtime
	net.SocketFunc = iface.Stack.Socket

	nic.Transport.EnableInterrupt(nic.IRQ, vnet.ReceiveQueue)
	startInterruptHandler(nic, iface)

	// ensure ISR is running before starting the interface
	for !signal.Waiting() {
		time.Sleep(1 * time.Millisecond)
	}

	go nic.Start()

	mac, _ := iface.Stack.HardwareAddress()

	if len(arg[3]) > 0 {
		ip, _, _ := strings.Cut(arg[0], `/`)

		log.Printf("network initialized (%s %s)\n", arg[0], mac)
		log.Printf("starting debug servers:\n")
		log.Printf("\thttp://%s:80/debug/pprof\n", ip)
		log.Printf("\tssh://%s:22\n", ip)

		go ssh.Start(Banner)
		go http.ListenAndServe(":80", nil)
	}

	// TODO: implement IRQ for UART driver
	log.Printf("stopping serial console\n")
	select {}

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
		Base: IOAPIC0_BASE,
	}

	ioapic.EnableInterrupt(dev.IRQ, dev.IRQ)

	// as IRQs are enabled, favor slicing dev.ReceiveWithHeader, opposed to
	// dev.Receive for better performance
	size := dev.HeaderLength + gnet.EthernetMaximumSize + gnet.MTU
	buf := make([]byte, size)

	isr := func(irq int) {
		switch irq {
		case dev.IRQ:
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

	go cpu.ServiceInterrupts(isr)
}
