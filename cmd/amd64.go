// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"runtime/goos"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi/x64"
)

func init() {
	shell.Add(shell.Cmd{
		Name: "info",
		Help: "device information",
		Fn:   infoCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "cpuid",
		Args:    2,
		Pattern: regexp.MustCompile(`^cpuid\s+([[:xdigit:]]+) ([[:xdigit:]]+)$`),
		Syntax:  "<leaf> <subleaf>",
		Help:    "show CPU capabilities",
		Fn:      cpuidCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "msr",
		Args:    1,
		Pattern: regexp.MustCompile(`^msr\s+([[:xdigit:]]+)$`),
		Syntax:  "<hex addr>",
		Help:    "read model-specific register",
		Fn:      msrCmd,
	})

	shell.Add(shell.Cmd{
		Name: "lspci",
		Help: "list PCI devices",
		Fn:   lspciCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "smp",
		Args:    1,
		Pattern: regexp.MustCompile(`^smp (\d+)$`),
		Syntax:  "<n>",

		Help: "launch SMP test",
		Fn:   smpCmd,
	})
}

func date(epoch int64) {
	x64.AMD64.SetTime(epoch)
}

func uptime() (ns int64) {
	return x64.AMD64.GetTime() - x64.AMD64.TimerOffset
}

func infoCmd(_ *shell.Interface, _ []string) (string, error) {
	var res bytes.Buffer

	ramStart, ramEnd := runtime.MemRegion()
	textStart, textEnd := runtime.TextRegion()
	_, heapStart := runtime.DataRegion()

	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	fmt.Fprintf(&res, "Runtime ......: %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(&res, "RAM ..........: %#08x-%#08x (%d MiB)\n", ramStart, ramEnd, (ramEnd-ramStart)/(1024*1024))
	fmt.Fprintf(&res, "Text .........: %#08x-%#08x\n", textStart, textEnd)
	fmt.Fprintf(&res, "Heap .........: %#08x-%#08x Alloc:%d MiB Sys:%d MiB\n", heapStart, ramEnd, m.HeapAlloc/(1024*1024), m.HeapSys/(1024*1024))
	fmt.Fprintf(&res, "CPU ..........: %s\n", x64.AMD64.Name())
	fmt.Fprintf(&res, "Cores ........: %d\n", amd64.NumCPU())
	fmt.Fprintf(&res, "Frequency ....: %v GHz\n", float32(x64.AMD64.Freq())/1e9)

	return res.String(), nil
}

func cpuidCmd(_ *shell.Interface, arg []string) (string, error) {
	var res bytes.Buffer

	leaf, err := strconv.ParseUint(arg[0], 16, 32)

	if err != nil {
		return "", fmt.Errorf("invalid leaf, %v", err)
	}

	subleaf, err := strconv.ParseUint(arg[1], 10, 32)

	if err != nil {
		return "", fmt.Errorf("invalid subleaf, %v", err)
	}

	eax, ebx, ecx, edx := x64.AMD64.CPUID(uint32(leaf), uint32(subleaf))

	fmt.Fprintf(&res, "EAX      EBX      ECX      EDX\n")
	fmt.Fprintf(&res, "%08x %08x %08x %08x\n", eax, ebx, ecx, edx)

	return res.String(), nil
}

func msrCmd(_ *shell.Interface, arg []string) (string, error) {
	var res bytes.Buffer

	addr, err := strconv.ParseUint(arg[0], 16, 64)

	if err != nil {
		return "", fmt.Errorf("invalid address, %v", err)
	}

	val := x64.AMD64.MSR(addr)
	fmt.Fprintf(&res, "%x", val)

	return res.String(), nil
}

func lspciCmd(_ *shell.Interface, arg []string) (string, error) {
	var res bytes.Buffer

	//fmt.Fprintf(&res, "Bus Vendor Device Revision Bar0\n")
	fmt.Fprintf(&res, "Bus Vendor Device Bar0\n")

	for i := range 256 {
		for _, d := range pci.Devices(i) {
			//fmt.Fprintf(&res, "%03d %04x   %04x   %02x   %#016x\n", i, d.Vendor, d.Device, d.Revision, d.BaseAddress(0))
			fmt.Fprintf(&res, "%03d %04x   %04x   %#016x\n", i, d.Vendor, d.Device, d.BaseAddress(0))
		}
	}

	return res.String(), nil
}

func smpCmd(console *shell.Interface, arg []string) (string, error) {
	var res bytes.Buffer
	var wg sync.WaitGroup
	var cc sync.Map
	var total int

	n, err := strconv.Atoi(arg[0])

	if err != nil {
		return "", fmt.Errorf("invalid goroutine count: %v", err)
	}

	ncpu := amd64.NumCPU()

	if goos.ProcID == nil || goos.Task == nil {
		return "", errors.New("no SMP detected")
	}

	fmt.Fprintf(console.Output, "%d cores detected, launching %d goroutines from CPU%2d\n", ncpu, n, goos.ProcID())

	start := time.Now()

	for i := 0; i < n; i++ {
		wg.Go(func() {
			cpu := goos.ProcID()

			for {
				if actual, loaded := cc.LoadOrStore(cpu, 1); loaded {
					if cc.CompareAndSwap(cpu, actual.(int), actual.(int)+1) {
						break
					}
				} else {
					break
				}
			}
		})
	}

	wg.Wait()
	elapsed := time.Since(start)

	cc.Range(func(cpu any, count any) bool {
		total += count.(int)
		fmt.Fprintf(&res, "CPU%2d %3d:%s\n", cpu.(uint64), count.(int), strings.Repeat("░", count.(int)))
		return true
	})

	fmt.Fprintf(&res, "Total %3d (%v)\n", total, elapsed)

	return res.String(), nil
}
