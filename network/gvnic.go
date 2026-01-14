// Copyright (c) The TamaGo Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package network

import (
	"fmt"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/kvm/gvnic"
	"github.com/usbarmory/tamago/soc/intel/ioapic"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/tamago-example/shell"
)

// chosen by the application for MSI-X signaling
const vector = 32

func Init(cpu *amd64.CPU, ioapic *ioapic.IOAPIC, console *shell.Interface) {
	gve := &gvnic.GVE{
		Device: pci.Probe(
			0,
			gvnic.PCI_VENDOR,
			gvnic.PCI_DEVICE,
		),
		IRQ: vector,
	}

	gve.Init()
	fmt.Printf("MAC: %x\n", gve.MAC)

	//if err := startNet(console, gve); err != nil {
	//	log.Printf("could not start networking, %v", err)
	//	return
	//}

	// This example illustrates IRQ handling, alternatively a poller can be
	// used with `gve.Start(true)`.
	//go func() {
		// On GCP we must ensure the ISR is running before starting the
		// interface.
	//	cpu.ClearInterrupt()
	//	gve.Start(true)
	//}()

	//transport.EnableInterrupt(VIRTIO_NET0_IRQ, gvnic.ReceiveQueue)
	//startInterruptHandler(gve, cpu, ioapic)
}
