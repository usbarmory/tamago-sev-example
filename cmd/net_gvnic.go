// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"regexp"
	"strings"

	"github.com/usbarmory/tamago/kvm/gvnic"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"

	"github.com/usbarmory/go-net"

	"github.com/usbarmory/tamago-sev-example/internal/ssh"
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
		Name:    "net-gve",
		Args:    3,
		Pattern: regexp.MustCompile(`^net-gve (\S+) (\S+)( debug)?$`),
		Syntax:  "<ip>       <gw> (debug)?",
		Help:    "start gVNIC networking",
		Fn:      gvnicCmd,
	})
}

// TODO: WiP
func gvnicCmd(_ *shell.Interface, arg []string) (res string, err error) {
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

	iface := gnet.Interface{
		NetworkDevice: gve,
	}

	if err := iface.Init(arg[0], gve.MAC().String(), arg[1]); err != nil {
		return "", fmt.Errorf("could not initialize networking, %v", err)
	}

	iface.HandleStackErr = func(err error, tx bool) {
		fmt.Printf("network stack error (tx:%v), %v", tx, err)
	}

	iface.Stack.EnableICMP()
	go iface.Start(context.Background())

	// hook interface into Go runtime
	net.SocketFunc = iface.Stack.Socket

	if len(arg[2]) > 0 {
		ip, _, _ := strings.Cut(arg[0], `/`)

		log.Printf("network initialized (%s %s)\n", arg[0], gve.MAC())
		log.Printf("starting debug servers:\n")
		log.Printf("\thttp://%s:80/debug/pprof\n", ip)
		log.Printf("\tssh://%s:22\n", ip)

		go ssh.Start(Banner)
		go http.ListenAndServe(":80", nil)
	}

	// required to avoid UEFI #VC handler overlap
	log.Printf("stopping serial console\n")
	select {}

	return fmt.Sprintf("network initialized (%s %s)\n", arg[0], gve.MAC()), nil
}
