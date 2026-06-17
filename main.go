// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build amd64

package main

import (
	"log"

	"github.com/usbarmory/tamago/soc/intel/ioapic"
	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi"
	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/usbarmory/tamago-sev-example/cmd"
	"github.com/usbarmory/tamago-sev-example/internal/kvm"
)

const (
	// Intel I/O Programmable Interrupt Controllers
	IOAPIC0_BASE = 0xfec00000
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
}

func main() {
	// disable UEFI watchdog
	x64.UEFI.Boot.SetWatchdogTimer(0)

	if sev.Features(x64.AMD64).SEV.SNP {
		if err := kvm.InitGHCB(); err != nil {
			log.Printf("could not initialize GHCB, %v", err)
		}
	} else {
		x64.AllocateDMA(10 << 20)
	}

	x64.InitSMP()

	console := &shell.Interface{
		Banner:  cmd.Banner,
		ReadWriter: x64.UART0,
	}

	// start interactive shell
	console.Start(true)

	if x64.Console.Out != 0 {
		x64.UEFI.Runtime.ResetSystem(uefi.EfiResetShutdown)
	}
}
