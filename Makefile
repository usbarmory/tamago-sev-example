# Copyright (c) The tamago-sev-example authors. All Rights Reserved.
#
# Use of this source code is governed by the license
# that can be found in the LICENSE file.

BUILD_TAGS = linkcpuinit,linkramsize,linkramstart,linkprintk
SHELL = /bin/bash
APP ?= tamago-sev-example

IMAGE_BASE := 10000000
TEXT_START := $(shell echo $$((16#$(IMAGE_BASE) + 16#10000)))
LDFLAGS := -E cpuinit -T $(TEXT_START) -R 0x1000 -X 'main.Console=${CONSOLE}'
GOFLAGS := -tags ${BUILD_TAGS} -trimpath -ldflags "${LDFLAGS}"
GOENV := GOOS=tamago GOARCH=amd64

OVMF ?= OVMF.fd
OVMFCODE ?= OVMF_CODE.fd
LOG ?= qemu.log

SMP ?= $(shell nproc)
QEMU ?= qemu-system-x86_64 -machine q35,pit=off,pic=off \
        -m 4G -smp $(SMP) \
        -enable-kvm -cpu host,invtsc=on,kvmclock=on -no-reboot \
        -device pcie-root-port,port=0x10,chassis=1,id=pci.0,bus=pcie.0,multifunction=on,addr=0x3 \
        -device virtio-net-pci,netdev=net0,mac=42:01:0a:84:00:02,disable-modern=true -netdev tap,id=net0,ifname=tap0,script=no,downscript=no \
        -drive file=fat:rw:$(CURDIR)/qemu-disk \
        -drive if=pflash,format=raw,readonly,file=$(OVMFCODE) \
        -global isa-debugcon.iobase=0x402 \
        -serial stdio -vga virtio \
        # -debugcon file:$(LOG)

QEMU-snp ?= qemu-system-x86_64 \
        -smp $(SMP) \
        -enable-kvm -cpu host,invtsc=on \
        -machine q35,confidential-guest-support=sev0,vmport=off,memory-backend=ram1 \
        -object memory-backend-memfd,id=ram1,size=4G,share=true,prealloc=false \
        -drive file=fat:rw:$(CURDIR)/qemu-disk \
        -bios $(OVMF) \
        -global isa-debugcon.iobase=0x402 \
        -serial stdio -nographic -monitor none \
        -object sev-snp-guest,id=sev0,cbitpos=51,reduced-phys-bits=1,policy=0xb0000
        # -debugcon file:$(LOG)

.PHONY: clean

#### primary targets ####

all: $(APP).efi

elf: $(APP)

efi: $(APP).efi

qemu: $(APP).efi
	mkdir -p $(CURDIR)/qemu-disk/efi/boot && cp $(CURDIR)/$(APP).efi $(CURDIR)/qemu-disk/efi/boot/bootx64.efi
	$(QEMU)

qemu-gdb: GOFLAGS := $(GOFLAGS:-w=)
qemu-gdb: GOFLAGS := $(GOFLAGS:-s=)
qemu-gdb: $(APP).efi
	mkdir -p $(CURDIR)/qemu-disk/efi/boot && cp $(CURDIR)/$(APP).efi $(CURDIR)/qemu-disk/efi/boot/bootx64.efi
	$(QEMU) -S -s

#### utilities ####

check_tamago:
	@if [ "${TAMAGO}" == "" ] || [ ! -f "${TAMAGO}" ]; then \
		echo 'You need to set the TAMAGO variable to a compiled version of https://github.com/usbarmory/tamago-go'; \
		exit 1; \
	fi

clean:
	@rm -fr $(APP) $(APP).efi $(CURDIR)/qemu-disk

#### dependencies ####

$(APP): check_tamago
	$(GOENV) $(TAMAGO) build $(GOFLAGS) -o ${APP}

$(APP).efi: $(APP)
	objcopy \
		--strip-debug \
		--target efi-app-x86_64 \
		--subsystem=efi-app \
		--image-base 0x$(IMAGE_BASE) \
		--stack=0x10000 \
		${APP} ${APP}.efi
	printf '\x26\x02' | dd of=${APP}.efi bs=1 seek=150 count=2 conv=notrunc,fsync # adjust Characteristics
