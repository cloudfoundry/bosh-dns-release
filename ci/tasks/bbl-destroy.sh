#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD

BOSH_BINARY_PATH=${BOSH_BINARY_PATH:-bosh}
ROOT_DIR=${ROOT_DIR:-$PWD}

REPO_DIR=$ROOT_DIR/envs

SKIP_GIT=${SKIP_GIT:-}

if [ -z "${SKIP_GIT}" ]; then
	REPO_DIR=$ROOT_DIR/envs-output

	git clone -q "file://$ROOT_DIR/envs" "$REPO_DIR"

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
fi

BBL_STATE_DIR=$REPO_DIR/$ENV_NAME

source $BBL_STATE_DIR/.envrc

$BOSH_BINARY_PATH deployments | awk '{ print $1 }' | xargs -n1 $BOSH_BINARY_PATH -n delete-deployment -d

$BOSH_BINARY_PATH -n clean-up --all

$BOSH_BINARY_PATH -n delete-env $BBL_STATE_DIR/bosh-manifest.yml \
  --vars-store $BBL_STATE_DIR/creds.yml  \
  --state $BBL_STATE_DIR/state.json \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars)

bbl --state-dir=$BBL_STATE_DIR destroy --no-confirm --skip-if-missing

if [ -z "${SKIP_GIT}" ]; then
  pushd $BBL_STATE_DIR
    git rm -rf .
  popd
fi
