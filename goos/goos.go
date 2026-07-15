// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package goos

import "unsafe"

// We use our own overlay, instead of github.com/usbarmory/tamago/goos, to
// override StackSystem as this is required to accomodate OVMF #VC handler
// stack usage.
const (
	ArenaBaseOffset     = 0
	HeapAddrBits        = 40
	LogHeapArenaBytes   = (2 + 20)
	LogPallocChunkPages = 9
	MinPhysPageSize     = 4096
	StackSystem         = 4096
)

var (
	RamStart       uint
	RamSize        uint
	RamStackOffset uint

	Bloc   uintptr
	Exit   func(code int32)
	Idle   func(until int64)
	ProcID func() uint64
	Task   func(sp, mp, gp, fn unsafe.Pointer)
	Wake   func(procid uint64)
)

func CPUinit()
func Hwinit0()
func InitRNG()
func GetRandomData(b []byte)
func Nanotime() int64
func Printk(c byte)
func Hwinit1()
