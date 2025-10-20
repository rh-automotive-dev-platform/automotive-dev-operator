# automotive-dev-operator

A simple controller that watches two CRs: `AutomotiveDev` which applies a pipeline and a few tasks. And `ImageBuild` which triggers an automotive OS image build, powered by automotive-image-builder.


## Description
An in-progress operator that deploys tasks for automotive-image-builder cloud building.
This includes a CLI tool and a webui

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/automotive-dev-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/automotive-dev-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Releases and Deployments (Tagged Versions)

This repository publishes versioned multi-arch images and a pinned installer manifest via GitHub Actions when you push a tag that starts with `v`.

### Create a release

1) Ensure CI variables and secrets are set in the repository:

- REGISTRY (repository variable): container registry hostname (e.g. `quay.io`)
- REPOSITORY (repository variable): org/namespace in the registry (e.g. `rh-sdv-cloud`)
- REGISTRY_USER (secret): registry username
- REGISTRY_PASSWORD (secret): registry password/token

2) Tag and push:

```sh
git tag v0.0.10
git push origin v0.0.10
```

On tag push, CI will:
- Retag existing multi-arch images (built on `main`) to `v1.2.3` for both:
  - `automotive-dev-operator`
  - `aib-webui`
- Build and attach `caib` CLI binaries for linux/amd64 and linux/arm64
- Generate and attach a pinned manifest: `install-v1.2.3.yaml`

### Deploy a specific version

Download the pinned manifest from the release and apply it:

```sh
TAG=v0.0.10
curl -L -o install-$TAG.yaml https://github.com/rh-automotive-dev-platform/automotive-dev-operator/releases/download/$TAG/install-$TAG.yaml
kubectl apply -f install-$TAG.yaml
kubectl apply -f config/samples/automotive_v1_automotivedev.yaml # to add the image building tasks
```

Verify rollout:

```sh
kubectl -n automotive-dev-operator-system get deploy -o custom-columns=NAME:.metadata.name,IMAGE:.spec.template.spec.containers[*].image
kubectl -n automotive-dev-operator-system rollout status deploy/ado-controller-manager
kubectl -n automotive-dev-operator-system rollout status deploy/ado-build-api
kubectl -n automotive-dev-operator-system rollout status deploy/ado-webui
```

To upgrade, re-apply with a newer `TAG`. To uninstall, run:

```sh
kubectl delete -f install-$TAG.yaml
```

### CAIB CLI (download and setup)

Download the CLI binary from the same release and install it in your PATH (Linux):

```bash
TAG=v0.0.11

curl -L -o caib-$TAG-$ARCH \
  https://github.com/rh-automotive-dev-platform/automotive-dev-operator/releases/download/$TAG/caib-$TAG-$ARCH

sudo install -m 0755 caib-$TAG-$ARCH /usr/local/bin/caib

# Verify
caib --version
```

Point the CLI to your Build API once (or pass `--server` each time):

```bash
export CAIB_SERVER="https://build-api.YOUR_DOMAIN"
```

See `cmd/caib/README.md` for full usage examples.

## Project Distribution

Following are the steps to build the installer and distribute this project to users.

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/automotive-dev-operator:tag
```

NOTE: The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without
its dependencies.

2. Using the installer

Users can just run kubectl apply -f <URL for YAML BUNDLE> to install the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/automotive-dev-operator/<tag or branch>/dist/install.yaml
```

Note: Prefer the release asset `install-<tag>.yaml` described above, which pins all component images to the specific version.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
