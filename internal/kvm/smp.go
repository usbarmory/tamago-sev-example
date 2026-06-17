// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kvm

import (
	"log"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/uefi/x64"
)

// TODO: WiP
func CreateAPs() {
	if GHCB == nil {
		return
	}

	ncpu := amd64.NumCPU()

	for i := 1; i < ncpu; i++ {
		if err := GHCB[0].RemoveAP(i); err != nil {
			log.Printf("could not stop AP%d, %v", i, err)
			return
		}

		// set VMSA defaults
		vmsa := &sev.VMSA{}
		vmsa.Init(0)

		// match current features
		vmsa.SEV_FEATURES = sev.Features(x64.AMD64).SEV.Features

		if err := GHCB[0].CreateAP(1, vmsa); err != nil {
			log.Printf("could not create AP%d, %v", 1, err)
			return
		}
	}

	x64.AMD64.InitSMP(-1)
}
