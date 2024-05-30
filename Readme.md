# Readme

This repository contains AusOcean's cloud services and support packages.

## Ocean Bench

Ocean Bench is AusOcean's cloud service for analyzing ocean data.

Instructions for building Ocean Bench can be found under cmd/oceanbench.

To deploy OceanBench:

```bash
gcloud app deploy --version V --project oceanbeach oceanbeach.yaml
```

Currently, OceanBench utilizes two datastores, namely NetReceiver's and VidGrind's.
Deploying datastore indexes therefore requires running two commands.

```bash
gcloud app deploy --version V --project netreceiver netreceiver_index.yaml
gcloud app deploy --version V --project vidgrind vidgrind_index.yaml
```
