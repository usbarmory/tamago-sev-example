// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"io"
	"log"
	"os"

	"github.com/usbarmory/go-boot/shell"

	"github.com/gliderlabs/ssh"
)

func sessionHandler(s ssh.Session) {
	c := &shell.Interface{
		Banner:     Banner,
		ReadWriter: s,
	}

	log.SetOutput(io.MultiWriter(os.Stdout, s))
	defer log.SetOutput(os.Stdout)

	c.Start(true)
}
