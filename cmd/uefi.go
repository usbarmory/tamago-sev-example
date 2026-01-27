// Copyright (c) The go-boot authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi"
	"github.com/usbarmory/go-boot/uefi/x64"
)

const (
	exitRetries   = 3
	maxVendorSize = 64
)

func init() {
	shell.Add(shell.Cmd{
		Name: "uefi",
		Help: "UEFI information",
		Fn:   uefiCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "cat",
		Args:    1,
		Pattern: regexp.MustCompile(`^cat (.*)`),
		Syntax:  "<path>",
		Help:    "show file contents",
		Fn:      catCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "ls",
		Args:    1,
		Pattern: regexp.MustCompile(`^ls(?: (\S+))?$`),
		Syntax:  "(<path>)?",
		Help:    "list directory contents",
		Fn:      lsCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "stat",
		Args:    1,
		Pattern: regexp.MustCompile(`^stat (.*)`),
		Syntax:  "<path>",
		Help:    "show file information",
		Fn:      statCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "reset",
		Args:    1,
		Pattern: regexp.MustCompile(`^reset(?: (cold|warm))?$`),
		Help:    "reset system",
		Syntax:  "(cold|warm)?",
		Fn:      resetCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "halt,shutdown",
		Args:    1,
		Pattern: regexp.MustCompile(`^(halt|shutdown)$`),
		Help:    "shutdown system",
		Fn:      shutdownCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "terminate",
		Help:    "exit EFI Boot Services",
		Fn:      terminateCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "efivar",
		Args:    1,
		Pattern: regexp.MustCompile(`^efivar(?: (verbose))?$`),
		Syntax:  "(verbose)?",
		Help:    "list all UEFI variables",
		Fn:      efivarCmd,
	})
}

func uefiCmd(_ *shell.Interface, _ []string) (res string, err error) {
	var buf bytes.Buffer
	var s []uint16

	t := x64.UEFI.SystemTable
	b := mem(uint(t.FirmwareVendor), maxVendorSize, nil)

	for i := 0; i < maxVendorSize; i += 2 {
		if b[i] == 0x00 && b[i+1] == 0 {
			break
		}

		s = append(s, binary.LittleEndian.Uint16(b[i:i+2]))
	}

	fmt.Fprintf(&buf, "UEFI Revision ......: %s\n", t.Revision())
	fmt.Fprintf(&buf, "Firmware Vendor ....: %s\n", string(utf16.Decode(s)))
	fmt.Fprintf(&buf, "Firmware Revision ..: %#x\n", t.FirmwareRevision)
	fmt.Fprintf(&buf, "Runtime Services  ..: %#x\n", t.RuntimeServices)
	fmt.Fprintf(&buf, "Boot Services ......: %#x\n", t.BootServices)

	return buf.String(), err
}

func catCmd(_ *shell.Interface, arg []string) (res string, err error) {
	if x64.Console.Out == 0 {
		return "", fmt.Errorf("EFI boot services not available")
	}

	root, err := x64.UEFI.Root()

	if err != nil {
		return "", fmt.Errorf("could not open root volume, %v", err)
	}

	arg[0] = strings.ReplaceAll(arg[0], `\`, `/`)
	buf, err := fs.ReadFile(root, arg[0])

	if err != nil {
		return "", fmt.Errorf("could not read file, %v", err)
	}

	return string(buf), nil
}

func lsCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var buf bytes.Buffer
	var info fs.FileInfo

	path := arg[0]

	if len(path) == 0 {
		path = "."
	}

	if x64.Console.Out == 0 {
		return "", fmt.Errorf("EFI boot services not available")
	}

	root, err := x64.UEFI.Root()

	if err != nil {
		return "", fmt.Errorf("could not open root volume, %v", err)
	}

	path = strings.ReplaceAll(path, `\`, `/`)
	entries, err := fs.ReadDir(root, path)

	if err != nil {
		return "", fmt.Errorf("could not read directory, %v", err)
	}

	for _, entry := range entries {
		if info, err = entry.Info(); err != nil {
			return
		}

		if info.IsDir() {
			fmt.Fprintf(&buf, "d ")
		} else {
			fmt.Fprintf(&buf, "f ")
		}

		fmt.Fprintf(&buf, "%s\n", entry.Name())
	}

	return buf.String(), err
}

func statCmd(_ *shell.Interface, arg []string) (res string, err error) {
	if x64.Console.Out == 0 {
		return "", fmt.Errorf("EFI boot services not available")
	}

	root, err := x64.UEFI.Root()

	if err != nil {
		return "", fmt.Errorf("could not open root volume, %v", err)
	}

	arg[0] = strings.ReplaceAll(arg[0], `\`, `/`)
	f, err := root.Open(arg[0])

	if err != nil {
		return "", fmt.Errorf("could not open file, %v", err)
	}

	defer f.Close()

	stat, err := f.Stat()

	if err != nil {
		return
	}

	buf := make([]byte, stat.Size())

	if _, err = f.Read(buf); err != nil {
		return "", fmt.Errorf("could not read file, %v", err)
	}

	return fmt.Sprintf("Size:%d ModTime:%s IsDir:%v Sys:%#x Sum256:%x\n",
		stat.Size(),
		stat.ModTime(),
		stat.IsDir(),
		stat.Sys(),
		sha256.Sum256(buf),
	), nil
}

func resetCmd(_ *shell.Interface, arg []string) (_ string, err error) {
	var resetType int

	switch arg[0] {
	case "cold":
		resetType = uefi.EfiResetCold
	case "warm", "":
		resetType = uefi.EfiResetWarm
	case "shutdown":
		resetType = uefi.EfiResetShutdown
	}

	log.Printf("performing system reset type %d", resetType)
	err = x64.UEFI.Runtime.ResetSystem(resetType)

	return
}

func shutdownCmd(_ *shell.Interface, _ []string) (_ string, err error) {
	return resetCmd(nil, []string{"shutdown"})
}

func terminateCmd(_ *shell.Interface, _ []string) (_ string, err error) {
	log.Print("exiting EFI boot services")

	if x64.Console.Out == 0 {
		return "", fmt.Errorf("EFI boot services not available")
	}

	for i := 0; i < exitRetries; i++ {
		if _, err = x64.UEFI.Boot.ExitBootServices(); err != nil {
			log.Print("exiting EFI boot services (retrying)")
			continue
		}
		break
	}

	if err != nil {
		return "", fmt.Errorf("could not exit EFI boot services, %v\n", err)
	}

	// silence EFI Simple Text console
	x64.Console.Out = 0

	return
}

func efivarCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var buf bytes.Buffer
	var guid uefi.GUID
	var name string

	verbose := arg[0] == "verbose"

	for {
		if err = x64.UEFI.Runtime.GetNextVariableName(&name, &guid); err != nil {
			break
		}

		fmt.Fprintf(&buf, "  %s %s\n", guid.String(), name)

		if !verbose {
			continue
		}

		attr, _, err := x64.UEFI.Runtime.GetVariable(name, guid, false)

		if err != nil {
			fmt.Fprintf(&buf, "    <could not obtain variable information>\n")
			continue
		}

		fmt.Fprintf(&buf, "    EFI_VARIABLE_NON_VOLATILE:                          %v\n", attr.NonVolatile)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_BOOTSERVICE_ACCESS:                    %v\n", attr.BootServiceAccess)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_RUNTIME_ACCESS:                        %v\n", attr.RuntimeServiceAccess)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_HARDWARE_ERROR_RECORD:                 %v\n", attr.HardwareErrorRecord)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_AUTHENTICATED_WRITE_ACCESS:            %v\n", attr.AuthWriteAccess)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_TIME_BASED_AUTHENTICATED_WRITE_ACCESS: %v\n", attr.TimeBasedAuthWriteAccess)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_APPEND_WRITE:                          %v\n", attr.AppendWrite)
		fmt.Fprintf(&buf, "    EFI_VARIABLE_ENHANCED_AUTHENTICATED_ACCESS:         %v\n", attr.EnhancedAuthAccess)
	}

	// fix-up error value as GetNextVariableName will return ErrEfiNotFound
	// if there are no more variables
	if errors.Is(err, uefi.ErrEfiNotFound) {
		err = nil
	}

	return buf.String(), err
}
