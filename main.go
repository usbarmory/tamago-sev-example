// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build amd64

package main

import (
	"log"
	"runtime"

	"github.com/usbarmory/go-boot/uefi/x64"
	"github.com/usbarmory/tamago/soc/intel/ioapic"

	"github.com/usbarmory/tamago-example/shell"
	"github.com/usbarmory/tamago-sev-example/cmd"
	"github.com/usbarmory/tamago-sev-example/network"
)

const (
	// Intel I/O Programmable Interrupt Controllers
	IOAPIC0_BASE = 0xfec00000

	// VirtIO Networking
	VIRTIO_NET_PCI_VENDOR = 0x1af4 // Red Hat, Inc.
	VIRTIO_NET_PCI_DEVICE = 0x1000 // Virtio 1.0 network device
)

const (
	exit        = true
	exitRetries = 3
)

var (
	// I/O APIC - GSI 0-23
	IOAPIC0 = &ioapic.IOAPIC{
		Index:   0,
		Base:    IOAPIC0_BASE,
		GSIBase: 0,
	}
)

func init() {
	log.SetFlags(0)
	log.SetOutput(x64.UART0)

	// 10MB
	x64.AllocateDMA(10 << 20)
}

func main() {
	var err error

	// disable UEFI watchdog
	x64.UEFI.Boot.SetWatchdogTimer(0)

	if exit {
		log.Print("exiting EFI boot services")

		for i := 0; i < exitRetries; i++ {
			if _, err = x64.UEFI.Boot.ExitBootServices(); err != nil {
				log.Print("exiting EFI boot services (retrying)")
				continue
			}
			break
		}

		if err != nil {
			log.Fatalf("could not exit EFI boot services, %v\n", err)
		}

		// silence EFI Simple Text console
		x64.Console.Out = 0
	}

	log.Printf("enable exceptions")
	x64.AMD64.EnableExceptions()

	//log.Printf("init SMP")
	//x64.AMD64.InitSMP(-1)

	log.Printf("init IOAPIC")
	IOAPIC0.Init()

	console := &shell.Interface{
		Banner:  cmd.Banner,
	}

	network.SetupStaticWebAssets(cmd.Banner)
	log.Printf("starting network")
	network.Init(x64.AMD64, IOAPIC0, VIRTIO_NET_PCI_VENDOR, VIRTIO_NET_PCI_DEVICE, console)

	log.Printf("done")
	runtime.Exit(0)
}
