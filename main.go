// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build amd64

package main

import (
	"log"

	"github.com/usbarmory/tamago/soc/intel/ioapic"

	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/usbarmory/tamago-example/shell"
	"github.com/usbarmory/tamago-sev-example/cmd"
)

const (
	// Intel I/O Programmable Interrupt Controllers
	IOAPIC0_BASE = 0xfec00000

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
}

func exitUEFI() {
	var err error

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

func main() {
	console := &shell.Interface{
		Banner:  cmd.Banner,
		ReadWriter: x64.UART0,
	}

	// disable UEFI watchdog
	x64.UEFI.Boot.SetWatchdogTimer(0)

	if exit {
		exitUEFI()
	}

	console.Start(true)

	log.Printf("exit")
}
