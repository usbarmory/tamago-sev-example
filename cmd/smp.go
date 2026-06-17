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
	"runtime/goos"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/usbarmory/tamago/amd64"

	"github.com/usbarmory/go-boot/shell"
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
