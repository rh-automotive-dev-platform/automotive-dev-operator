#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(dirname "$(realpath "$0")")
ROOT_DIR=$SCRIPT_DIR/..

# Get version from Makefile
VERSION=$(grep "^VERSION " "$ROOT_DIR/Makefile" | head -1 | cut -d'=' -f2 | xargs)
IMAGE_TAG_BASE=$(grep "^IMAGE_TAG_BASE " "$ROOT_DIR/Makefile" | head -1 | cut -d'=' -f2 | xargs)

BUNDLE_REPO="${IMAGE_TAG_BASE}-bundle"
CATALOG_REPO="${IMAGE_TAG_BASE}-catalog"

echo "Building FBC for automotive-dev-operator v${VERSION}"
echo "Bundle: ${BUNDLE_REPO}:v${VERSION}"
echo "Catalog: ${CATALOG_REPO}:v${VERSION}"

TEMPLATE="
schema: olm.template.basic
entries:
  - schema: olm.package
    name: automotive-dev-operator
    defaultChannel: alpha
  - schema: olm.channel
    package: automotive-dev-operator
    name: alpha
    entries:
      - name: automotive-dev-operator.v${VERSION}
  - schema: olm.bundle
    image: ${BUNDLE_REPO}:v${VERSION}
"

mkdir -p "$ROOT_DIR/catalog"
echo "$TEMPLATE" | "$ROOT_DIR/bin/opm" alpha render-template basic -oyaml >| "$ROOT_DIR/catalog/automotive-dev-operator.yaml"

echo "FBC generated at catalog/automotive-dev-operator.yaml"

echo "Building catalog image..."
podman build -t "${CATALOG_REPO}:v${VERSION}" -f "$ROOT_DIR/catalog.Dockerfile" "$ROOT_DIR"

echo "Catalog image built: ${CATALOG_REPO}:v${VERSION}"
echo ""
echo "To push the catalog, run:"
echo "  podman push ${CATALOG_REPO}:v${VERSION}"

