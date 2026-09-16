// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package irq

import (
	"log"
	"runtime/goos"

	"github.com/usbarmory/tamago/soc/intel/ioapic"

	"github.com/usbarmory/go-boot/uefi/x64"
)

const (
	IOAPIC0_BASE = 0xfec00000
	COM1_IRQ     = 33
)

func StartHandler(id int, fn func()) {
	cpu := x64.AMD64

	if cpu.LAPIC != nil {
		cpu.LAPIC.Enable()
	}

	ioapic := &ioapic.IOAPIC{
		Base: IOAPIC0_BASE,
	}

	ioapic.EnableInterrupt(id, id)
	ioapic.EnableInterrupt(x64.UART0.IRQ, COM1_IRQ)

	ch := make(chan bool)
	x64.UART0.EnableInterrupt(ch)

	isr := func(irq int) {
		switch irq {
		case id:
			fn()
		//case COM1_IRQ:
		//	ch <- true
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
