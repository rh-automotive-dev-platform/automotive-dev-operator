#!/bin/sh
set -ex

if [ "$(params.export-format)" = "image" ]; then
  file_extension=".raw"
elif [ "$(params.export-format)" = "qcow2" ]; then
  file_extension=".qcow2"
else
  file_extension="$(params.export-format)"
fi

exportFile=$(params.distro)-$(params.target)-$(params.export-format)${file_extension}

echo "Pushing image to $(params.repository-url)"
oras push --disable-path-validation \
  $(params.repository-url) \
  $exportFile:application/vnd.oci.image.layer.v1.tar

echo "Image pushed successfully to registry"
