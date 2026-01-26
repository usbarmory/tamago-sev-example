// Copyright (c) The go-boot authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"fmt"
	_ "net/http/pprof"
	"regexp"

	"github.com/usbarmory/tamago-example/shell"
)

func init() {
	shell.Add(shell.Cmd{
		Name:    "virtio-net",
		Args:    4,
		Pattern: regexp.MustCompile(`^virtio-net (\S+) (\S+) (\S+)( debug)?$`),
		Syntax:  "<ip> <mac> <gw> (debug)?",
		Help:    "start VirtIO networking",
		Fn:      virtioNetCmd,
	})
}

func virtioNetCmd(_ *shell.Interface, arg []string) (res string, err error) {
	return "", fmt.Errorf("could not locate network protocol, %v", err)
}
