// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build ignore

package cmd

import (
	"fmt"
	_ "net/http/pprof"

	"github.com/usbarmory/tamago/kvm/gvnic"
	"github.com/usbarmory/tamago/kvm/sev"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/go-boot/uefi/x64"
)

// Google Virtual Private Cloud (GCP) - europe-west3
const (
	MAC     = "42:01:0a:84:00:02"
	Netmask = "255.255.255.0"
	IP      = "10.156.0.2"
	Gateway = "10.156.0.1"
)

func init() {
	shell.Add(shell.Cmd{
		Name: "net-gve",
		Help: "start gVNIC networking",
		Fn:   gvnicCmd,
	})
}

func gvnicCmd(console *shell.Interface, arg []string) (res string, err error) {
	if !sev.Features(x64.AMD64).SEV.SEV {
		x64.AllocateDMA(10 << 20)
	} else if err = initGHCB(); err != nil {
		return "", fmt.Errorf("could not initialize GHCB, %v", err)
	}

	gve := &gvnic.GVE{
		Device: pci.Probe(
			0,
			gvnic.PCI_VENDOR,
			gvnic.PCI_DEVICE,
		),
	}

	if err = gve.Init(); err != nil {
		return "", fmt.Errorf("%+v %v", gve.Info, err)
	}

	return fmt.Sprintf("network initialized (%s)\n", gve.MAC()), nil
}
