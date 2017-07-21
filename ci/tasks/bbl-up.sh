#!/bin/bash -eux

set -eu -o pipefail

BOSH_BINARY_PATH=${BOSH_BINARY_PATH:-bosh}
ROOT_DIR=${ROOT_DIR:-$PWD}
BOSH_DEPLOYMENT_PATH=${BOSH_DEPLOYMENT_PATH:-$ROOT_DIR/bosh-deployment}
BOSH_RELEASE_PATH=${BOSH_RELEASE_PATH:-$ROOT_DIR/bosh-candidate-release/bosh-dev-release.tgz}

REPO_DIR=$ROOT_DIR/envs

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
		git commit -m "$ENV_NAME: bbl-up"
	}

	trap commit_state EXIT
fi

BBL_STATE_DIR=$REPO_DIR/$ENV_NAME
mkdir -p $BBL_STATE_DIR

bbl --state-dir=$BBL_STATE_DIR up --no-director

$BOSH_BINARY_PATH int $BOSH_DEPLOYMENT_PATH/bosh.yml \
  --vars-store $BBL_STATE_DIR/creds.yml  \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars) \
  -o $BOSH_DEPLOYMENT_PATH/local-bosh-release.yml \
  -o $BOSH_DEPLOYMENT_PATH/local-dns.yml \
  -o $BOSH_DEPLOYMENT_PATH/gcp/cpi.yml  \
  -o $BOSH_DEPLOYMENT_PATH/external-ip-not-recommended.yml \
  -o $BOSH_DEPLOYMENT_PATH/jumpbox-user.yml \
  -v local_bosh_release=$BOSH_RELEASE_PATH \
  > $BBL_STATE_DIR/bosh-manifest.yml

$BOSH_BINARY_PATH create-env $BBL_STATE_DIR/bosh-manifest.yml \
  --state $BBL_STATE_DIR/state.json \
  --vars-store $BBL_STATE_DIR/creds.yml \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars)

$BOSH_BINARY_PATH int $BBL_STATE_DIR/creds.yml --path /jumpbox_ssh/private_key > $BBL_STATE_DIR/jumpbox.key
chmod 600 $BBL_STATE_DIR/jumpbox.key

cat > $BBL_STATE_DIR/.envrc <<EOF
export BOSH_CLIENT=admin
export BOSH_CLIENT_SECRET="$($BOSH_BINARY_PATH int $BBL_STATE_DIR/creds.yml --path /admin_password)"
export BOSH_ENVIRONMENT="$(bbl --state-dir=$BBL_STATE_DIR director-address)"
export BOSH_CA_CERT="$($BOSH_BINARY_PATH int $BBL_STATE_DIR/creds.yml --path /director_ssl/ca)"
export BOSH_GW_USER=jumpbox
export BOSH_GW_PRIVATE_KEY=jumpbox.key
EOF

source $BBL_STATE_DIR/.envrc

$BOSH_BINARY_PATH -n update-cloud-config <(bbl --state-dir=$BBL_STATE_DIR cloud-config)
