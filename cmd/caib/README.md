# caib — Cloud Automotive Image Builder CLI

`caib` is a CLI that talks to the Automotive Dev Build API to create, monitor, and download image builds.

## Installation

Build from source (requires Go):

```bash
make build
# or
go build -o bin/caib ./cmd/caib
```

## Quick start

Set the API endpoint once (or pass `--server` on every command):

```bash
export CAIB_SERVER=https://your-build-api.example
```

Create a build, follow logs, and download the artifact when complete:

```bash
bin/caib build \
  --name my-build \
  --manifest simple.aib.yml \
  --arch arm64 \
  --export-format image \
  --follow --download
```

List builds:

```bash
bin/caib list --server "$CAIB_SERVER"
```

Download an artifact for an existing completed build:

```bash
bin/caib download --name my-build --output-dir ./output --server "$CAIB_SERVER"
```

## Commands and flags

### build
Creates an `ImageBuild` and optionally waits, follows logs, and downloads artifacts.

Required input:
- `--server` or `CAIB_SERVER`: Base URL of the Build API (e.g., `https://api.example`).
- `--name`: Unique build name.
- `--manifest`: Path to a local AIB manifest (`*.aib.yml` or `*.mpp.yml`).

Common options:
- `--distro`: Distro (default: `cs9`).
- `--target`: Target platform (default: `qemu`).
- `--arch`: Architecture, e.g., `arm64` or `amd64` (default: `arm64`).
- `--mode`: Build mode (default: `image`).
- `--export-format`: `image` (raw) or `qcow2` (default: `image`).
- `--automotive-image-builder`: Container image for AIB (default: `quay.io/centos-sig-automotive/automotive-image-builder:1.0.0`).
- `--storage-class`: Storage class to use for build workspace PVC (optional).
- `--define`: Repeatable `KEY=VALUE` custom definitions passed to AIB.
- `--aib-args`: Extra arguments passed to AIB (space-separated string).
- `--wait` (`-w`): Wait for build to complete.
- `--follow` (`-f`): Stream build logs (retries transient 503/504).
- `--download` (`-d`): Download artifact when done.
- `--timeout`: Minutes to wait when `--wait` is used (default: 60).

Behavior:
- Local file references in the manifest are detected and uploaded automatically right after the build is accepted.
  - Supported manifest keys: `content.add_files[].source` and `content.add_files[].source_path` (also under `qm.content.add_files`).
  - Relative `source` entries are rewritten to `source_path` under `/workspace/shared`.
  - Relative `source_path` entries are normalized to `/workspace/shared/...`.
- Upload waits for the server’s “Uploading” phase and retries while the upload pod becomes ready.
- Log following uses the Build API logs endpoint and retries on 503/504.

Examples:

```bash
# Build qcow2 with a custom AIB image and extra args
bin/caib build \
  --name demo-qcow2 \
  --manifest simple.aib.yml \
  --arch arm64 \
  --export-format qcow2 \
  --automotive-image-builder quay.io/centos-sig-automotive/automotive-image-builder:latest \
  --aib-args "--fusa" \
  --follow --download
```

### download
Downloads the artifact of a completed build via the Build API.

Flags:
- `--server` or `CAIB_SERVER`
- `--name` (required)
- `--output-dir` (default: `./output`)

### list
Lists existing builds.

Flags:
- `--server` or `CAIB_SERVER`

## Manifest notes

- Relative `source` and `source_path` entries are supported in `content.add_files` and `qm.content.add_files`.
  - During preprocessing, they are rewritten to absolute paths under `/workspace/shared` inside the build pod.
- Example snippet:

```yaml
content:
  add_files:
    - path: /etc/containers/systemd/radio.container
      source_path: radio.container  # local file next to the manifest
```

## Known behaviors and timeouts

- Upload readiness: The CLI waits up to 10 minutes for the upload pod and retries uploads on 503 (Service Unavailable).
- Log follow: If the log stream endpoint returns 503/504 early in the build, the CLI keeps retrying; once logs are available you will see “Streaming logs…”.
- Build wait: `--wait` obeys `--timeout` (minutes). Increase it for large builds (e.g., `--timeout 120`).

## Environment variables

- `CAIB_SERVER`: Base URL of the Build API (equivalent to `--server`).

## Exit codes

- Non-zero on validation errors, upload errors (after retries), or when the build ends in a Failed phase.

## Troubleshooting

- “upload pod not ready” or HTTP 503 during upload: The CLI will retry automatically. If persistent, verify cluster capacity and that the operator can create the upload pod.
- “504 Gateway Timeout” during log follow: Usually transient while the build pod is starting. The CLI will keep retrying.
- Build fails quickly after upload: The controller may still be transitioning the PVC; re-run with a larger `--timeout` and check operator logs.

## Version

Print version:

```bash
bin/caib --version
```

## License
Apache-2.0
