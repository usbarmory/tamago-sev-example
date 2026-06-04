// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"regexp"
	"runtime/goos"

	"github.com/google/go-sev-guest/verify"

	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi"
	"github.com/usbarmory/go-boot/uefi/x64"
)

var (
	snp      *uefi.SNPConfigurationTable
	features *sev.SVMFeatures
	secrets  *sev.SecretsPage
	ghcb     map[uint64]*sev.GHCB
)

func init() {
	if !sev.Features(x64.AMD64).SEV.SEV {
		return
	}

	shell.Add(shell.Cmd{
		Name: "sev",
		Help: "AMD SEV-SNP information",
		Fn:   sevCmd,
	})

	shell.Add(shell.Cmd{
		Name:    "sev-report",
		Args:    1,
		Pattern: regexp.MustCompile(`^sev-report(?: (raw|verify))?$`),
		Syntax:  "(raw|verify)?",
		Help:    "AMD SEV-SNP attestation report",
		Fn:      attestationCmd,
	})

	shell.Add(shell.Cmd{
		Name: "sev-kdf",
		Help: "AMD SEV-SNP key derivation",
		Fn:   kdfCmd,
	})

	shell.Add(shell.Cmd{
		Name: "sev-tsc",
		Help: "AMD SEV-SNP TSC information",
		Fn:   tscCmd,
	})
}

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
	return x64.AMD64.SetEncryptedBit(start, end, features.EncryptedBit, false)
}

func InitGHCB() (err error) {
	if ghcb != nil {
		return
	}

	if features = sev.Features(x64.AMD64); !features.SEV.SNP {
		return
	}

	if x64.Console.Out == 0 {
		return fmt.Errorf("EFI boot services not available")
	}

	if snp, err = x64.UEFI.GetSNPConfiguration(); err != nil {
		return fmt.Errorf(" could not find AMD SEV-SNP pages, %v", err)
	}

	secrets = &sev.SecretsPage{}

	if err = secrets.Init(uint(snp.SecretsPagePhysicalAddress), int(snp.SecretsPageSize)); err != nil {
		return fmt.Errorf(" could not initialize AMD SEV-SNP secrets, %v", err)
	}

	// map GHCB <> vCPU
	ghcb = make(map[uint64]*sev.GHCB)

	// initialize vCPU0 GHCB for DMA allocation
	ghcb[0] = &sev.GHCB{
		CPU: x64.AMD64,
	}

	// allocate unencrypted region for GHCB.GuestRequest and driver use
	if err = initSharedDMA(ghcb[0], 10 << 20); err != nil {
		return fmt.Errorf("could not allocate shared DMA region, %v", err)
	}

	// finish VCPUs instance creation

	ghcb[0].Region = dma.Default()

	for n := uint64(1); n < uint64(amd64.NumCPU()); n++ {
		ghcb[n] = &sev.GHCB{
			CPU: x64.AMD64,
		}
		ghcb[n].Region = dma.Default()
	}

	return
}

func sevCmd(_ *shell.Interface, _ []string) (res string, err error) {
	var buf bytes.Buffer

	defer func() {
		res = buf.String()
		err = nil
	}()

	if features == nil {
		return "", fmt.Errorf("AMD SEV-SNP features not available")
	}

	fmt.Fprintf(&buf, "SEV ................: %v\n", features.SEV.SEV)
	fmt.Fprintf(&buf, "SEV-ES .............: %v\n", features.SEV.ES)
	fmt.Fprintf(&buf, "SEV-SNP ............: %v\n", features.SEV.SNP)
	fmt.Fprintf(&buf, "Encrypted bit ......: %d\n", features.EncryptedBit)

	if !features.SEV.SNP {
		return
	}

	if x64.Console.Out == 0 {
		return "", fmt.Errorf("EFI boot services not available")
	}

	if snp, err = x64.UEFI.GetSNPConfiguration(); err != nil {
		fmt.Fprintf(&buf, " could not find AMD SEV-SNP pages, %v", err)
		return
	}

	fmt.Fprintf(&buf, "SNP Version ........: %d\n\n", snp.Version)
	fmt.Fprintf(&buf, "Secrets Page .......: %#x (%d bytes)\n", snp.SecretsPagePhysicalAddress, snp.SecretsPageSize)

	if secrets == nil {
		return "", fmt.Errorf("AMD SEV-SNP secrets not available")
	}

	fmt.Fprintf(&buf, "Secrets Version ....: %d\n", secrets.Version)
	fmt.Fprintf(&buf, "TSC Factor .........: %#x\n", secrets.TSCFactor)
	fmt.Fprintf(&buf, "Launch Mitigations .: %#x\n", secrets.LaunchMitVector)
	fmt.Fprintf(&buf, "VMPCK0 .............: %#02x -- %#02x\n", secrets.VMPCK0[0], secrets.VMPCK0[31])
	fmt.Fprintf(&buf, "VMPCK1 .............: %#02x -- %#02x\n", secrets.VMPCK1[0], secrets.VMPCK1[31])
	fmt.Fprintf(&buf, "VMPCK2 .............: %#02x -- %#02x\n", secrets.VMPCK2[0], secrets.VMPCK2[31])
	fmt.Fprintf(&buf, "VMPCK3 .............: %#02x -- %#02x\n", secrets.VMPCK3[0], secrets.VMPCK3[31])
	fmt.Fprintf(&buf, "VMPCK3 .............: %#02x -- %#02x\n", secrets.VMPCK3[0], secrets.VMPCK3[31])

	fmt.Fprintf(&buf, "\n")
	fmt.Fprintf(&buf, "vCPU ...............: %d\n", goos.ProcID())
	fmt.Fprintf(&buf, "GHCB GPA ...........: %#x\n", x64.AMD64.MSR(sev.MSR_AMD_GHCB))

	hvFeatures, err := ghcb[goos.ProcID()].HypervisorFeatures()

	if err != nil {
		fmt.Fprintf(&buf, " could not request hypervisor featuress, %v", err)
		return
	}

	fmt.Fprintf(&buf, "Hypervisor Features : %#x\n", hvFeatures)

	return
}

func attestationCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var buf bytes.Buffer
	var report *sev.AttestationReport

	if ghcb == nil {
		return "", fmt.Errorf("GHCB not present")
	}

	data := make([]byte, 64)
	rand.Read(data)

	if report, err = ghcb[goos.ProcID()].GetAttestationReport(data, secrets.VMPCK0[:], 0); err != nil {
		return "", fmt.Errorf("could not get report, %v", err)
	}

	if arg[0] == "raw" {
		return fmt.Sprintf("%x", report.Bytes()), nil
	}

	fmt.Fprintf(&buf, "Version ............: %x\n", report.Version)
	fmt.Fprintf(&buf, "VMPL ...............: %x\n", report.VMPL)
	fmt.Fprintf(&buf, "SignatureAlgo ......: %x\n", report.SignatureAlgo)
	fmt.Fprintf(&buf, "CurrentTCB .........: %x\n", report.CurrentTCB)
	fmt.Fprintf(&buf, "Measurement ........: %x\n", report.Measurement)
	fmt.Fprintf(&buf, "ReportedTCB ........: %x\n", report.ReportedTCB)
	fmt.Fprintf(&buf, "CommittedTCB .......: %x\n", report.CommittedTCB)
	fmt.Fprintf(&buf, "Launch  Mitigations : %#x\n", report.LaunchMitVector)
	fmt.Fprintf(&buf, "Current Mitigations : %#x\n", report.CurrentMitVector)
	fmt.Fprintf(&buf, "SignatureR .........: %x\n", report.Signature[0:48])
	fmt.Fprintf(&buf, "SignatureS .........: %x\n", report.Signature[72:72+48])

	if arg[0] == "verify" {
		if net.SocketFunc == nil {
			fmt.Fprintf(&buf, "Verification errors : network unavailable\n")
		} else {
			log.Printf("** requesting on-line report verification in background **")
			go func() {
				err := verify.RawSnpReport(report.Bytes(), verify.DefaultOptions())
				log.Printf("Verification result : %v\n", err)
			}()
		}
	}

	return buf.String(), nil
}

func kdfCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var key []byte

	if ghcb == nil {
		return "", fmt.Errorf("GHCB not present")
	}

	// Perform key derivation from the physical CPU keys, bound to the
	// Guest Policy and VM identity.
	req := &sev.KeyRequest{
		KeySelect:        sev.RootKeySelVCEK,
		GuestFieldSelect: sev.GuestSVN | sev.Measurement | sev.GuestPolicy,
	}

	if key, err = ghcb[goos.ProcID()].DeriveKey(req, secrets.VMPCK0[:], 0); err != nil {
		return "", fmt.Errorf("could not derive key, %v", err)
	}

	return fmt.Sprintf("%x\n", key), nil
}

func tscCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var buf bytes.Buffer
	var tsc *sev.TSCInfo

	if ghcb == nil {
		return "", fmt.Errorf("GHCB not present")
	}

	if tsc, err = ghcb[goos.ProcID()].TSCInfo(secrets.VMPCK0[:], 0); err != nil {
		return "", fmt.Errorf("could not request TSC, %v", err)
	}

	fmt.Fprintf(&buf, "Guest TSC Scale ....: %d\n", tsc.GuestTSCScale)
	fmt.Fprintf(&buf, "Guest TSC Offset ...: %d\n", tsc.GuestTSCOffset)
	fmt.Fprintf(&buf, "TSC Factor .........: %d\n", tsc.TSCFactor)

	return buf.String(), nil
}
