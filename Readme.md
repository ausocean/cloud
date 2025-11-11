# Readme

This repository contains AusOcean's cloud services and support packages.

## Ocean Bench

Ocean Bench is AusOcean's cloud service for analyzing ocean data.

Instructions for building Ocean Bench can be found under cmd/oceanbench.

To deploy OceanBench:

```bash
gcloud app deploy --version V --project oceanbeach oceanbeach.yaml
```

Currently, OceanBench utilizes datastores in two projects, namely ausocean and vidgrind.
Deploying datastore indexes therefore requires running two commands.

```bash
cp vidgrind_index.yaml index.yaml
gcloud app deploy --project vidgrind index.yaml

cp ausocean_index.yaml index.yaml
gcloud app deploy --project ausocean index.yaml
```

To clean up indexes:

```bash
cp vidgrind_index.yaml index.yaml
gcloud datastore indexes cleanup --project vidgrind index.yaml
```
