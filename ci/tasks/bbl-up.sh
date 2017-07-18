#!/bin/bash -eux

set -eu -o pipefail

ROOT_DIR=$PWD
REPO_DIR=$ROOT_DIR/envs-output

git clone -q "file://$ROOT_DIR/envs" "$REPO_DIR"

BBL_STATE_DIR=$REPO_DIR/$ENV_NAME

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

mkdir -p $BBL_STATE_DIR

bbl --state-dir=$BBL_STATE_DIR up --no-director

bosh int $ROOT_DIR/bosh-deployment/bosh.yml \
  --vars-store $BBL_STATE_DIR/creds.yml  \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars) \
  -o $ROOT_DIR/bosh-deployment/local-bosh-release.yml \
  -o $ROOT_DIR/bosh-deployment/local-dns.yml \
  -o $ROOT_DIR/bosh-deployment/gcp/cpi.yml  \
  -o $ROOT_DIR/bosh-deployment/external-ip-not-recommended.yml \
  -o $ROOT_DIR/bosh-deployment/jumpbox-user.yml \
  -v local_bosh_release=$ROOT_DIR/bosh-candidate-release/bosh-dev-release.tgz \
  > $BBL_STATE_DIR/bosh-manifest.yml

bosh create-env $BBL_STATE_DIR/bosh-manifest.yml \
  --state $BBL_STATE_DIR/state.json \
  --vars-store $BBL_STATE_DIR/creds.yml \
  -l <(bbl --state-dir=$BBL_STATE_DIR bosh-deployment-vars)

cat > $BBL_STATE_DIR/bosh.sh <<EOF
export BOSH_CLIENT=admin
export BOSH_CLIENT_SECRET="$(bosh int $BBL_STATE_DIR/creds.yml --path /admin_password)"
export BOSH_ENVIRONMENT="$(bbl --state-dir=$BBL_STATE_DIR director-address)"
export BOSH_CA_CERT="$(bosh int $BBL_STATE_DIR/creds.yml --path /director_ssl/ca)"
EOF

source $BBL_STATE_DIR/bosh.sh

bosh -n update-cloud-config <(bbl --state-dir=$BBL_STATE_DIR cloud-config)
