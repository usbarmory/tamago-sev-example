#!/bin/bash

export BUCKET=tamago-sev-example-bucket

gcloud storage rm gs://$BUCKET/compressed-image.tar.gz
gcloud storage buckets delete gs://$BUCKET
gcloud compute instances stop tamago-sev-example --zone europe-west3-a
gcloud compute instances delete tamago-sev-example --zone europe-west3-a
gcloud compute images delete tamago-sev-example
