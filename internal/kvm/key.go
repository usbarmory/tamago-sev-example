// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kvm

import (
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"runtime/goos"

	"filippo.io/keygen"
	"golang.org/x/crypto/ssh"

	"github.com/usbarmory/tamago/kvm/sev"
)

// Signer derives a signer uniquely and deterministically generated for this VM
// for attestation purposes.
func Signer() (deviceKey ssh.Signer, err error) {
	if GHCB == nil {
		return nil, fmt.Errorf("GHCB not present")
	}

	// Perform key derivation from the physical CPU keys, bound to the
	// Guest Policy and VM identity.
	req := &sev.KeyRequest{
		KeySelect:        sev.RootKeySelVCEK,
		GuestFieldSelect: sev.GuestSVN | sev.Measurement | sev.GuestPolicy,
	}

	ghcb := GHCB[goos.ProcID()]
	vmpck := Secrets.VMPCK0[:]

	key, err := ghcb.DeriveKey(req, vmpck, 0)

	if err != nil {
		return nil, fmt.Errorf("could not derive key, %v", err)
	}

	if key, err = hkdf.Key(sha256.New, key, nil, "ssh-host-key/ecdsa-p256/v1", sha256.BlockSize); err != nil {
		return nil, fmt.Errorf("could not perform hkdf, %v", err)
	}

	pk, err := keygen.ECDSA(elliptic.P256(), key)

	if err != nil {
		return nil, fmt.Errorf("could not perform keygen, %v", err)
	}

	der, err := x509.MarshalECPrivateKey(pk)

	if err != nil {
		return nil, fmt.Errorf("could not marshal key, %v", err)
	}

	pemBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}

	return ssh.ParsePrivateKey(pem.EncodeToMemory(pemBlock))
}
