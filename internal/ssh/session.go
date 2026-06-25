// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ssh

import (
	"io"
	"log"
	"os"

	"github.com/gliderlabs/ssh"

	"github.com/usbarmory/go-boot/shell"
	"github.com/usbarmory/tamago-sev-example/internal/kvm"
)

func Start(banner string) {
	srv := &ssh.Server{
		Addr: ":22",
	}

	signer, err := kvm.Signer()

	if err != nil {
		log.Printf("could not create signer, %v", err)
	} else {
		srv.AddHostKey(signer)
	}

	ssh.Handle(func(s ssh.Session) {
		c := &shell.Interface{
			Banner:     banner,
			ReadWriter: s,
		}

		log.SetOutput(io.MultiWriter(os.Stdout, s))
		defer log.SetOutput(os.Stdout)

		c.Start(true)
	})

	if err := srv.ListenAndServe(); err != nil {
		log.Printf("ssh server terminated, %v", err)
	}
}
