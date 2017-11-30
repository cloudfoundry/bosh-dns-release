#!/bin/bash
set -euxo pipefail

function kill_bbl_ssh {
  # kill the ssh tunnel to jumpbox, set up by bbl env
  # (or task will hang forever)
  pkill ssh || true
}

trap kill_bbl_ssh EXIT

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}

set +x
eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
set -x

# Ensure the environment is clean
bosh -n -d bosh delete-deployment
bosh -n -d docker delete-deployment

# 7. Clean-up old artifacts
bosh -n clean-up --all
