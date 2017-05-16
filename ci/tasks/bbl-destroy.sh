#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/bbl-state

export PATH=$BBL_STATE_DIR/bin:$PATH
source $BBL_STATE_DIR/bosh.sh

set +u
source /usr/local/share/chruby/chruby.sh
chruby ruby-2.3.1
set -u

bosh deployments | awk '{ print $1 }' | xargs -I name bosh -n -d name delete-deployment

bosh -n delete-env $BBL_STATE_DIR/bosh-manifest.yml \
  --vars-store $BBL_STATE_DIR/creds.yml  \
  --state $BBL_STATE_DIR/state.json \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars)

bbl --state-dir=$BBL_STATE_DIR destroy --no-confirm --skip-if-missing
