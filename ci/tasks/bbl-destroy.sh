#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD
REPO_DIR=$ROOT_DIR/envs-output

git clone -q "file://$ROOT_DIR/envs" "$REPO_DIR"

BBL_STATE_DIR=$REPO_DIR/$ENV_NAME

cd "$BBL_STATE_DIR"

function commit_state {
  cd "$REPO_DIR"

  if [[ ! -n "$(git status --porcelain)" ]]; then
    return
  fi

  git config user.name "${GIT_COMMITTER_NAME:-CI Bot}"
  git config user.email "${GIT_COMMITTER_EMAIL:-ci@localhost}"
  git add -A .
  git commit -m "$ENV_NAME: bbl-destroy"
}

trap commit_state EXIT

source $BBL_STATE_DIR/bosh.sh

bosh deployments | awk '{ print $1 }' | xargs -I name bosh -n -d name delete-deployment

bosh -n clean-up --all

bosh -n delete-env $BBL_STATE_DIR/bosh-manifest.yml \
  --vars-store $BBL_STATE_DIR/creds.yml  \
  --state $BBL_STATE_DIR/state.json \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars)

bbl --state-dir=$BBL_STATE_DIR destroy --no-confirm --skip-if-missing

git rm -rf .
