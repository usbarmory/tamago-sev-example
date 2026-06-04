// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"runtime/goos"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi/x64"
)

func init() {
	shell.Add(shell.Cmd{
		Name:    "smp",
		Args:    1,
		Pattern: regexp.MustCompile(`^smp (\d+)$`),
		Syntax:  "<n>",

		Help: "launch SMP test",
		Fn:   smpCmd,
	})
}

// TODO: WiP
func createAPs() {
	if ghcb == nil {
		return
	}

	ncpu := amd64.NumCPU()

	for i := 1; i < ncpu; i++ {
		if err := ghcb[0].RemoveAP(i); err != nil {
			log.Printf("could not stop AP%d, %v", i, err)
			return
		}

		// set VMSA defaults
		vmsa := &sev.VMSA{}
		vmsa.Init(0)

		// match current features
		vmsa.SEV_FEATURES = sev.Features(x64.AMD64).SEV.Features

		if err := ghcb[0].CreateAP(1, vmsa); err != nil {
			log.Printf("could not create AP%d, %v", 1, err)
			return
		}
	}

	x64.AMD64.InitSMP(-1)
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
