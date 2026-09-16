// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	"net"
	"os/signal"
	"regexp"
	"time"

	"github.com/usbarmory/tamago/kvm/gvnic"
	"github.com/usbarmory/tamago/soc/intel/pci"

	"github.com/usbarmory/go-boot/shell"

	"github.com/usbarmory/go-net"

	"github.com/usbarmory/tamago-sev-example/internal/irq"
)

// Google Virtual Private Cloud (GCP) - europe-west3
const (
	MAC     = "42:01:0a:84:00:02"
	Netmask = "255.255.255.0"
	IP      = "10.156.0.2"
	Gateway = "10.156.0.1"

	// redirection vectors for MSI-X signal
	GVE_IRQ = 32
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

	iface := &gnet.Interface{
		NetworkDevice: gve,
	}

	if err := iface.Init(arg[0], gve.MAC().String(), arg[1]); err != nil {
		return "", fmt.Errorf("could not initialize networking, %v", err)
	}

	iface.HandleStackErr = func(err error, tx bool) {
		fmt.Printf("network stack error (tx:%v), %v", tx, err)
	}

	iface.Stack.EnableICMP()

	// hook interface into Go runtime
	net.SocketFunc = iface.Stack.Socket

	isr := func() {
		size := gnet.EthernetMaximumSize + gnet.MTU
		buf := make([]byte, size)

		defer gve.ClearInterrupt(gvnic.RX)

		for {
			if n, err := gve.Receive(buf); err != nil || n == 0 {
				return
			}

			iface.Stack.RecvInboundPacket(buf)
		}
	}

	if err = gve.EnableInterrupt(GVE_IRQ, gvnic.RX); err != nil {
		return
	}

	irq.StartHandler(GVE_IRQ, isr)

	// ensure ISR is running before starting the interface
	for !signal.Waiting() {
		time.Sleep(1 * time.Millisecond)
	}

	if len(arg[2]) > 0 {
		startDebugServices(arg[0], gve.MAC())
	}

	// start RX events
	gve.ClearInterrupt(gvnic.RX)

	return fmt.Sprintf("network initialized (%s %s)\n", arg[0], gve.MAC()), nil
}
