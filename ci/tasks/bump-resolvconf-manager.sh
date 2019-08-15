#!/usr/bin/env bash

set -eux

git config --global user.email "ci@localhost"
git config --global user.name "CI Bot"

pushd bosh-dns-release/
  cat >> config/private.yml <<EOF
---
blobstore:
  provider: s3
  options:
    access_key_id: "$BLOBSTORE_ACCESS_KEY_ID"
    secret_access_key: "$BLOBSTORE_SECRET_ACCESS_KEY"
EOF

  bosh add-blob ../resolvconf-manager/resolvconf-manager resolvconf-manager/resolvconf-manager
  bosh upload-blobs

  if [ -z "$(git status --porcelain)" ]; then
    exit
  fi

  git add -A
  git commit -m "Bump resolvconf-manager blob"
popd
