#!/usr/bin/env bash

set -eu

task_dir=$PWD
repo_output=$task_dir/bosh-dns-release-output

git config --global user.email "ci@localhost"
git config --global user.name "CI Bot"

git clone bosh-dns-release "$repo_output"

cd "$repo_output"

cat >> config/private.yml <<EOF
---
blobstore:
  provider: s3
  options:
    access_key_id: "$BLOBSTORE_ACCESS_KEY_ID"
    secret_access_key: "$BLOBSTORE_SECRET_ACCESS_KEY"
EOF

bosh vendor-package golang-1-linux "$task_dir/golang-release"
bosh vendor-package golang-1-windows "$task_dir/golang-release"

if [ -z "$(git status --porcelain)" ]; then
  exit
fi

git add -A

git commit -m "Update golang packages from golang-release"
