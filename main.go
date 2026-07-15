// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build amd64

package main

import (
	"log"

	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi"
	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/usbarmory/tamago-sev-example/cmd"
	"github.com/usbarmory/tamago-sev-example/internal/kvm"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(x64.UART0)
}

func main() {
	// disable UEFI watchdog
	x64.UEFI.Boot.SetWatchdogTimer(0)

	// This unikernel does not set its own (*GHCB).Layout and therefore
	// relies on the previous GHCB GPA as initialized by OVMF as well as
	// its #VC handler.
	//
	// For this reason any GHCB call through github.com/usbarmory/kvm/sev
	// might overlap with OVMF #VC handling and result in an error
	// (particularly under SMP).
	if sev.Features(x64.AMD64).SEV.SNP {
		if err := kvm.InitGHCB(); err != nil {
			log.Printf("could not initialize GHCB, %v", err)
		}
	} else {
		x64.AllocateDMA(10 << 20)
	}

	x64.InitSMP()

	console := &shell.Interface{
		Banner:     cmd.Banner,
		ReadWriter: x64.UART0,
	}

	// start interactive shell
	console.Start(true)

	if x64.Console.Out != 0 {
		x64.UEFI.Runtime.ResetSystem(uefi.EfiResetShutdown)
	}
}
