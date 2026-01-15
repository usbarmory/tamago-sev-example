#!/bin/bash

export BUCKET=tamago-sev-example-bucket
export MNT=tamago-sev-example-mnt

make efi
dd if=/dev/zero of=disk.raw bs=1M count=100 conv=fsync
sudo losetup -f disk.raw
sudo mkfs.vfat -F 32 /dev/loop0
sudo mount /dev/loop0 $MNT
sudo mkdir -p $MNT/EFI/BOOT
sudo cp tamago-sev-example.efi $MNT/EFI/BOOT/BOOTX64.EFI
sudo cp tamago-sev-example.efi $MNT/EFI/BOOT/shimx64.efi
sudo umount -l $MNT
sudo losetup -d /dev/loop0
sudo sync
tar -cvzf tamago-sev-disk.raw.tar.gz disk.raw

tar --format=oldgnu -Sczf compressed-image.tar.gz disk.raw
gcloud storage buckets create gs://$BUCKET
gcloud storage cp compressed-image.tar.gz gs://$BUCKET
gcloud compute images create tamago-sev-example --source-uri gs://$BUCKET/compressed-image.tar.gz --architecture=X86_64 --guest-os-features=UEFI_COMPATIBLE,SEV_SNP_CAPABLE
gcloud compute instances create tamago-sev-example --zone=europe-west3-a --machine-type=n2d-standard-4 --metadata="serial-port-enable=1" --image tamago-sev-example --confidential-compute-type=SEV_SNP --min-cpu-platform="AMD Milan" --maintenance-policy=TERMINATE --private-network-ip 10.156.0.2
