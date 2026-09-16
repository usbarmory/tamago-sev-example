// Copyright (c) The tamago-sev-example authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package cmd

import (
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/usbarmory/tamago-sev-example/internal/ssh"
)

func startDebugServices(cidr string, mac net.HardwareAddr) {
	ip, _, _ := strings.Cut(cidr, `/`)

	log.Printf("starting debug servers:\n")
	log.Printf("\thttp://%s:80/debug/pprof\n", ip)
	log.Printf("\tssh://%s:22\n", ip)

	go ssh.Start(Banner)
	go http.ListenAndServe(":80", nil)
}
