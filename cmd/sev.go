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

	"github.com/usbarmory/tamago/kvm/sev"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi/x64"

	"github.com/usbarmory/tamago-sev-example/internal/kvm"
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

func reportVerify(report *sev.AttestationReport) {
	if net.SocketFunc == nil {
		log.Printf("Verification error, network unavailable")
		return
	}

	log.Printf("** requesting on-line report verification **")

	go func() {
		if err := verify.RawSnpReport(report.Bytes(), verify.DefaultOptions()); err != nil {
			log.Printf("Verification error, %v", err)
		} else {
			log.Printf("Verification succeeded")
		}
	}()
}

func sevCmd(_ *shell.Interface, _ []string) (res string, err error) {
	var buf bytes.Buffer

	defer func() {
		res = buf.String()
		err = nil
	}()

	if kvm.Features == nil {
		return "", fmt.Errorf("AMD SEV-SNP features not available")
	}

	fmt.Fprintf(&buf, "SEV ................: %v\n", kvm.Features.SEV.SEV)
	fmt.Fprintf(&buf, "SEV-ES .............: %v\n", kvm.Features.SEV.ES)
	fmt.Fprintf(&buf, "SEV-SNP ............: %v\n", kvm.Features.SEV.SNP)
	fmt.Fprintf(&buf, "Encrypted bit ......: %d\n", kvm.Features.EncryptedBit)

	if !kvm.Features.SEV.SNP {
		return
	}

	if x64.Console.Out == 0 {
		return "", fmt.Errorf("EFI boot services not available")
	}

	snp, err := x64.UEFI.GetSNPConfiguration()

	if err != nil {
		fmt.Fprintf(&buf, " could not find AMD SEV-SNP pages, %v", err)
		return
	}

	fmt.Fprintf(&buf, "SNP Version ........: %d\n\n", snp.Version)
	fmt.Fprintf(&buf, "Secrets Page .......: %#x (%d bytes)\n", snp.SecretsPagePhysicalAddress, snp.SecretsPageSize)

	s := kvm.Secrets

	if s == nil {
		return "", fmt.Errorf("AMD SEV-SNP secrets not available")
	}

	fmt.Fprintf(&buf, "Secrets Version ....: %d\n", s.Version)
	fmt.Fprintf(&buf, "TSC Factor .........: %#x\n", s.TSCFactor)
	fmt.Fprintf(&buf, "Launch Mitigations .: %#x\n", s.LaunchMitVector)
	fmt.Fprintf(&buf, "VMPCK0 .............: %#02x -- %#02x\n", s.VMPCK0[0], s.VMPCK0[31])
	fmt.Fprintf(&buf, "VMPCK1 .............: %#02x -- %#02x\n", s.VMPCK1[0], s.VMPCK1[31])
	fmt.Fprintf(&buf, "VMPCK2 .............: %#02x -- %#02x\n", s.VMPCK2[0], s.VMPCK2[31])
	fmt.Fprintf(&buf, "VMPCK3 .............: %#02x -- %#02x\n", s.VMPCK3[0], s.VMPCK3[31])

	fmt.Fprintf(&buf, "\n")
	fmt.Fprintf(&buf, "vCPU ...............: %d\n", goos.ProcID())
	fmt.Fprintf(&buf, "GHCB GPA ...........: %#x\n", x64.AMD64.MSR(sev.MSR_AMD_GHCB))

	hvFeatures, err := kvm.GHCB[goos.ProcID()].HypervisorFeatures()

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

	if kvm.GHCB == nil {
		return "", fmt.Errorf("GHCB not present")
	}

	ghcb := kvm.GHCB[goos.ProcID()]
	vmpck := kvm.Secrets.VMPCK0[:]

	data := make([]byte, 64)
	rand.Read(data)

	if report, err = ghcb.GetAttestationReport(data, vmpck, 0); err != nil {
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
		reportVerify(report)
	}

	return buf.String(), nil
}

func kdfCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var key []byte

	if kvm.GHCB == nil {
		return "", fmt.Errorf("GHCB not present")
	}

	// Perform key derivation from the physical CPU keys, bound to the
	// Guest Policy and VM identity.
	req := &sev.KeyRequest{
		KeySelect:        sev.RootKeySelVCEK,
		GuestFieldSelect: sev.GuestSVN | sev.Measurement | sev.GuestPolicy,
	}

	ghcb := kvm.GHCB[goos.ProcID()]
	vmpck := kvm.Secrets.VMPCK0[:]

	if key, err = ghcb.DeriveKey(req, vmpck, 0); err != nil {
		return "", fmt.Errorf("could not derive key, %v", err)
	}

	return fmt.Sprintf("%x\n", key), nil
}

func tscCmd(_ *shell.Interface, arg []string) (res string, err error) {
	var buf bytes.Buffer
	var tsc *sev.TSCInfo

	if kvm.GHCB == nil {
		return "", fmt.Errorf("GHCB not present")
	}

	ghcb := kvm.GHCB[goos.ProcID()]
	vmpck := kvm.Secrets.VMPCK0[:]

	if tsc, err = ghcb.TSCInfo(vmpck, 0); err != nil {
		return "", fmt.Errorf("could not request TSC, %v", err)
	}

	fmt.Fprintf(&buf, "Guest TSC Scale ....: %d\n", tsc.GuestTSCScale)
	fmt.Fprintf(&buf, "Guest TSC Offset ...: %d\n", tsc.GuestTSCOffset)
	fmt.Fprintf(&buf, "TSC Factor .........: %d\n", tsc.TSCFactor)

	return buf.String(), nil
}
