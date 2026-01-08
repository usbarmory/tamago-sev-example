// Copyright (c) The TamaGo Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package network

import (
	"log"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/kvm/virtio"
	"github.com/usbarmory/tamago/soc/intel/ioapic"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/tamago-example/shell"

	"github.com/usbarmory/virtio-net"
)

// chosen by the application for MSI-X signaling
const VIRTIO_NET0_IRQ = 32

func Init(cpu *amd64.CPU, ioapic *ioapic.IOAPIC, vendor uint16, device uint16, console *shell.Interface) {
	transport := &virtio.LegacyPCI{
		Device: pci.Probe(0, vendor, device),
	}

	dev := &vnet.Net{
		Transport:    transport,
		IRQ:          VIRTIO_NET0_IRQ,
		HeaderLength: 10,
	}

	if err := startNet(console, dev); err != nil {
		log.Printf("could not start networking, %v", err)
		return
	}

	// This example illustrates IRQ handling, alternatively a poller can be
	// used with `dev.Start(true)`.
	go func() {
		// On GCP we must ensure the ISR is running before starting the
		// interface.
		cpu.ClearInterrupt()
		dev.Start(true)
	}()

	transport.EnableInterrupt(VIRTIO_NET0_IRQ, vnet.ReceiveQueue)
	startInterruptHandler(dev, cpu, ioapic)
}
