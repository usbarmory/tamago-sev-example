Introduction
============

This project showcases a network [TamaGo](https://github.com/usbarmory/tamago)
UEFI unikernel for execution under [AMD Secure Encrypted Virtualization
(SEV)](https://www.qemu.org/docs/master/system/i386/amd-memory-encryption.html)
using [QEMU](https://www.qemu.org/) or [Google Compute
Engine](https://cloud.google.com/products/compute).

Operation
=========

The default operation is to exit UEFI boot services, then present a shell and
its services.

```
initializing EFI services
exiting EFI boot services

tamago-sev-example • tamago/amd64 (go1.26rc2) • UEFI x64

build						 # build information
cpuid		<leaf> <subleaf>		 # show CPU capabilities
date		(time in RFC339 format)?	 # show/change runtime date and time
dns		<host>				 # resolve domain
exit,quit					 # exit application
help						 # this help
info						 # device information
lspci						 # list PCI devices
msr		<hex addr>			 # read model-specific register
net		<ip> <mac> <gw> (debug)?	 # start UEFI networking
peek		<hex addr> <size>		 # memory display (use with caution)
poke		<hex addr> <hex value>		 # memory write   (use with caution)
sev						 # AMD SEV-SNP information
sev-kdf						 # AMD SEV-SNP key derivation
sev-report	(raw|verify)?			 # AMD SEV-SNP attestation report
sev-tsc						 # AMD SEV-SNP TSC information
stack						 # goroutine stack trace (current)
stackall					 # goroutine stack trace (all)
uptime						 # show system running time
virtio-net	<ip> <mask> <gw> (debug)?	 # start VirtIO networking

> sev
SEV ................: true
SEV-ES .............: true
SEV-SNP ............: true
Encrypted bit ......: 51
SNP Version ........: 1

Secrets Page .......: 0x80d000 (4096 bytes)
Secrets Version ....: 4
TSC Factor .........: 0xc8
Launch Mitigations .: 0xb
VMPCK0 .............: 0xd9 -- 0x0d
VMPCK1 .............: 0x1a -- 0xb6
VMPCK2 .............: 0x98 -- 0xe5
VMPCK3 .............: 0xd8 -- 0x56
```

Compiling
=========

Build the [TamaGo compiler](https://github.com/usbarmory/tamago-go)
(or use the [latest binary release](https://github.com/usbarmory/tamago-go/releases/latest)):

```
wget https://github.com/usbarmory/tamago-go/archive/refs/tags/latest.zip
unzip latest.zip
cd tamago-go-latest/src && ./all.bash
cd ../bin && export TAMAGO=`pwd`/go
```

The following environment variables configure the `tamago-sev-example.efi`
executable build:

* `IMAGE_BASE`: must be set (in hex) within a memory range
  available in the target UEFI environment for the unikernel allocation, the
  [HCL](https://github.com/usbarmory/go-boot/wiki#hardware-compatibility-list) or
  `memmap` command from an [UEFI Shell](https://github.com/pbatard/UEFI-Shell)
  can provide such value, when empty a common default value is set.

Build the `tamago-sev-example.efi` executable:

```
git clone https://github.com/usbarmory/tamago-sev-example && cd tamago-sev-example
make efi IMAGE_BASE=10000000
```

Emulated hardware with QEMU
===========================

QEMU supported targets can be executed under emulation, using the
[Open Virtual Machine Firmware](https://github.com/tianocore/tianocore.github.io/wiki/OVMF)
as follows:

```
make qemu OVMF=<path to OVMF.fd>
```

For networking, `tap0` should be configured as follows (Linux example):

```
ip tuntap add dev tap0 mode tap group <your user group>
ip addr add 10.0.0.2/24 dev tap0
ip link set tap0 up
```

Confidential VMs
----------------

The `qemu-snp` target provides an example of execution under
[AMD Secure Encrypted Virtualization (SEV)](https://www.qemu.org/docs/master/system/i386/amd-memory-encryption.html)
and can be used on [compatible hardware](https://www.amd.com/en/developer/sev.html).

Cloud deployments
=================

The following example demonstrates how to create, and deploy, a UEFI-bootable
image for cloud deployments:

* [Google Compute Engine](https://github.com/usbarmory/go-boot/wiki/Google-Compute-Engine)
* [Google Compute Engine - Confidential VM (AMD SEV-SNP)](https://github.com/usbarmory/go-boot/wiki/Google-Compute-Engine-(AMD-SEV%E2%80%90SNP))

Networking
==========

The following sections illustrate the network options available depending on
the KVM configuration.

For all `net-*` commands the optional `debug` strings can be passed as final
argument to enable Go [profiling server](https://pkg.go.dev/net/http/pprof) and
an unauthenticated SSH console exposing the unikernel shell.

```
> net-virtio 10.0.0.1 255.255.255.0 10.0.0.2 debug
starting debug servers:
        http://10.0.0.1:80/debug/pprof
        ssh://10.0.0.1:22
network initialized (10.0.0.1/24 da:e7:ac:e2:5e:05)

> dns golang.org
[142.251.209.17 2a00:1450:4002:410::2011]
```

VirtIO networking
-----------------

When running under any QEMU target, VirtIO networking is available through the
`net-virtio` command.

The command takes an IP address, a network mask, and a gateway IP address as
arguments.

```
> net-virtio 10.0.0.1 255.255.255.0 10.0.0.2
> network initialized (10.0.0.1 42010a840002)
```

UEFI networking
---------------

When running under QEMU with the unikernel loaded from a disk image (e.g. `make
qemu` or `make qemu-snp-disk` targets), UEFI Simple Nework Protocol is
available through the `net-uefi` command.

The command takes an IP address in CIDR notation, a fixed MAC address or `:` to
automatically generate a random MAC, and a gateway IP address as arguments.

```
> net-uefi 10.0.0.1/24 : 10.0.0.2
network initialized (10.0.0.1/24 da:e7:ac:e2:5e:05)
```

Google Virtual NIC (gVNIC)
--------------------------

> [!WARNING] this is a work in progress, not yet operational

When running under  [Google Compute
Engine](https://cloud.google.com/products/compute) gVNIC support is available
through the `net-gve` command.

Debugging
=========

An emulated target can be [debugged with GDB](https://retrage.github.io/2019/12/05/debugging-ovmf-en.html/)
using `make qemu-gdb`, this will make qemu waiting for a GDB connection that
can be launched as follows:

```
gdb -ex "target remote 127.0.0.1:1234"
```

Breakpoints can be set in the usual way:

```
b cpuinit
continue
```

License
=======

tamago-sev-example | https://github.com/usbarmory/tamago-sev-example
Copyright (c) The tamago-sev-example authors. All Rights Reserved.

These source files are distributed under the BSD-style license found in the
[LICENSE](https://github.com/usbarmory/tamago-sev-example/blob/main/LICENSE) file.
