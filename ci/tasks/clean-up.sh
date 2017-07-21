#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/envs/$ENV_NAME

source $BBL_STATE_DIR/.envrc

bosh -n clean-up --all

bosh deployments | awk '{ print $1 }' | xargs -I name bosh -n -d name delete-deployment
