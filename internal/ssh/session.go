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

const addr = ":22"

func Start(banner string) {
	ssh.Handle(func(s ssh.Session) {
		c := &shell.Interface{
			Banner:     banner,
			ReadWriter: s,
		}

		log.SetOutput(io.MultiWriter(os.Stdout, s))
		defer log.SetOutput(os.Stdout)

		c.Start(true)
	})

	srv := &ssh.Server{
		Addr: addr,
	}

	signer, err := kvm.Signer()

	if err != nil {
		// use random host key
		err = ssh.ListenAndServe(addr, nil)
	} else {
		// use VM unique key
		srv.AddHostKey(signer)
		err = srv.ListenAndServe()
	}

	log.Printf("ssh server terminated, %v", err)
}
