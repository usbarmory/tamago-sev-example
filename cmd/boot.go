// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"strconv"

	"github.com/usbarmory/armory-boot/exec"
	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi"
	"github.com/usbarmory/go-boot/uefi/x64"
	"github.com/usbarmory/tamago/dma"
)

func init() {
	shell.Add(shell.Cmd{
		Name:    "go",
		Args:    3,
		Pattern: regexp.MustCompile(`^go (\S+) ([[:xdigit:]]+) (\d+)$`),
		Syntax:  "<path> <hex addr> <size>",
		Help:    "load and start TamaGo EFI unikernel",
		Fn:      goCmd,
	})
}

func boot(image *exec.ELFImage, start uint, size int) (err error) {
	if image.Region, err = dma.NewRegion(start, size, false); err != nil {
		return errors.New("could not reserve memory for unikernel loading")
	}

	image.Region.Reserve(size, 0)
	// release in case of error
	defer image.Region.Release(image.Region.Start())

	// reserve runtime memory for kernel loading
	if err = x64.UEFI.Boot.AllocatePages(
		uefi.AllocateAddress,
		uefi.EfiLoaderData,
		size,
		uint64(start),
	); err != nil {
		return errors.New("could not allocate memory for unikernel loading")
	}

	// load kernel in reserved memory
	if err = image.Load(); err != nil {
		return fmt.Errorf("could not load unikernel, %v", err)
	}

	handle := x64.UEFI.ImageHandle()
	table := x64.UEFI.Address()

	log.Printf("booting unikernel@%#x handle:%x table:%x", image.Entry(), handle, table)
	return image.BootEFI(uint(handle), uint(table), nil)
}

func goCmd(_ *shell.Interface, arg []string) (res string, err error) {
	path := arg[0]

	start, err := strconv.ParseUint(arg[1], 16, dma.DefaultAlignment*8)

	if err != nil {
		return "", fmt.Errorf("invalid address, %v", err)
	}

	size, err := strconv.ParseUint(arg[2], 10, 32)

	if err != nil {
		return "", fmt.Errorf("invalid size, %v", err)
	}

	root, err := x64.UEFI.Root()

	if err != nil {
		return "", fmt.Errorf("could not open root volume, %v", err)
	}

	log.Printf("loading EFI unikernel %s", path)

	buf, err := fs.ReadFile(root, arg[0])

	if err != nil {
		return "", fmt.Errorf("could not read file, %v", err)
	}

	image := &exec.ELFImage{
		ELF: buf,
	}

	return "", boot(image, uint(start), int(size))
}
