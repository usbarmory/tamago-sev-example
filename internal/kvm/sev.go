// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kvm

import (
	"fmt"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/uefi/x64"
)

var (
	Features *sev.SVMFeatures
	Secrets  *sev.SecretsPage
	GHCB     map[uint64]*sev.GHCB
)

func initSharedDMA(ghcb *sev.GHCB, dmaSize int) (err error) {
	// align to 2MB page
	dmaStart := int(x64.RamSize) - dmaSize
	dmaSize += dmaStart % (2 << 20)

	if err = x64.AllocateDMA(dmaSize); err != nil {
		return
	}

	start := uint64(dma.Default().Start())
	end := uint64(dma.Default().End())

	// Invalidate memory before clearing encryption, do it in chunks to
	// avoid dealing with hypervisor RMP fragmentation.
	chunk := sev.MaxPSCEntries * uint64(4096)

	for s := start; s < end; s += chunk {
		if err = ghcb.PageStateChange(start, start+chunk, sev.PAGE_SIZE_4K, false); err != nil {
			return
		}
	}

	// disable encryption for DMA region
	return x64.AMD64.SetEncryptedBit(start, end, Features.EncryptedBit, false)
}

func InitGHCB() (err error) {
	if GHCB != nil {
		return
	}

	if Features = sev.Features(x64.AMD64); !Features.SEV.SNP {
		return
	}

	if x64.Console.Out == 0 {
		return fmt.Errorf("EFI boot services not available")
	}

	snp, err := x64.UEFI.GetSNPConfiguration()

	if err != nil {
		return fmt.Errorf(" could not find AMD SEV-SNP pages, %v", err)
	}

	Secrets = &sev.SecretsPage{}

	if err = Secrets.Init(uint(snp.SecretsPagePhysicalAddress), int(snp.SecretsPageSize)); err != nil {
		return fmt.Errorf(" could not initialize AMD SEV-SNP secrets, %v", err)
	}

	// map GHCB <> vCPU
	GHCB = make(map[uint64]*sev.GHCB)

	// initialize vCPU0 GHCB for DMA allocation
	GHCB[0] = &sev.GHCB{
		CPU: x64.AMD64,
	}

	// allocate unencrypted region for GHCB.GuestRequest and driver use
	if err = initSharedDMA(GHCB[0], 10<<20); err != nil {
		return fmt.Errorf("could not allocate shared DMA region, %v", err)
	}

	// finish VCPUs instance creation

	GHCB[0].Region = dma.Default()

	for n := uint64(1); n < uint64(amd64.NumCPU()); n++ {
		GHCB[n] = &sev.GHCB{
			CPU: x64.AMD64,
		}
		GHCB[n].Region = dma.Default()
	}

	return
}
