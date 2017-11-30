#!/bin/bash
set -euxo pipefail

export BBL_STATE_DIR=$PWD/bbl-state/${BBL_STATE_SUBDIRECTORY}

set +x
eval "$(bbl print-env --state-dir=$BBL_STATE_DIR)"
set -x

# Ensure the environment is clean
bosh -n -d bosh delete-deployment
bosh -n -d docker delete-deployment

# 7. Clean-up old artifacts
bosh -n clean-up --all
