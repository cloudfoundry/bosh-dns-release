#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/envs/$ENV_NAME

source $BBL_STATE_DIR/.envrc

bosh -n clean-up --all

bosh deployments --column name | xargs -n1 bosh -n delete-deployment -d
