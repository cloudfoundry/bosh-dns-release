#!/usr/bin/env bash

set -eux

VERSION=$(cat version/number)
cp version/number bumped-version/number

export ROOT_PATH=$PWD
PROMOTED_REPO=$PWD/final-bosh-dns-release

export DEV_RELEASE_PATH=$ROOT_PATH/candidate-release/bosh*.tgz

git clone ./bosh-dns-release $PROMOTED_REPO

pushd $PROMOTED_REPO
  git status

  cat >> config/private.yml <<EOF
---
blobstore:
  provider: s3
  options:
    access_key_id: "$BLOBSTORE_ACCESS_KEY_ID"
    secret_access_key: "$BLOBSTORE_SECRET_ACCESS_KEY"
EOF

  bosh finalize-release --version $VERSION $DEV_RELEASE_PATH

  git add -A
  git status

  git config --global user.email "ci@localhost"
  git config --global user.name "CI Bot"

  git commit -m "Adding final release $VERSION via concourse"

popd

cat <<EOF >tag/tag-name
v${VERSION}
EOF

cat <<EOF >tag/annotate-msg
Final release $VERSION tagged via concourse
EOF
