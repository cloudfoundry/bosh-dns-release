#!/bin/bash

set -e -o pipefail
set -x

function realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}
function cleanup() {
  rm -rf $temp_env_file
  rm -rf $BOSH_RELEASE_DIR
  rm -rf $BOSH_STEMCELL_DIR
  rm -rf $BOSH_CLI_DIR
  rm -rf $BBL_CLI_DIR
}

trap cleanup EXIT

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

fly -t production login

temp_env_file=$(mktemp bbl.env)
chmod +x $temp_env_file
cat <<EOF >$temp_env_file
export BBL_GCP_SERVICE_ACCOUNT_KEY=$(lpass show 3654688481222762882 --notes | yq .bbl_gcp_service_account_key_id)
export BBL_GCP_PROJECT_ID=cf-bosh-core
export BBL_GCP_ZONE=us-central1-a
export BBL_GCP_REGION=us-central1
export BBL_IAAS=gcp
EOF

BOSH_CLI_DIR=$(mktemp -d bosh-cli)
pushd $BOSH_CLI_DIR
  curl -o bosh-cli-linux https://s3.amazonaws.com/bosh-cli-alpha-artifacts/alpha-bosh-cli-0.0.250-linux-amd64
popd

BOSH_RELEASE_DIR=$(mktemp -d bosh-release)
pushd $BOSH_RELEASE_DIR
  curl -L -J -o bosh-dev-release.tgz https://bosh.io/d/github.com/cloudfoundry/bosh?v=261.4
popd

BOSH_STEMCELL_DIR=$(mktemp -d bosh-stemcell)
pushd $BOSH_STEMCELL_DIR
  curl -L -J -o bosh-stemcell-3362-warden-boshlite-ubuntu-trusty-go_agent.tgz https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent?v=3363.22
popd

BBL_CLI_DIR=$(mktemp -d bbl-cli)
pushd $BBL_CLI_DIR
  curl -o bbl-v3.2.0_linux_x86-64 -L -v https://github.com/cloudfoundry/bosh-bootloader/releases/download/v3.2.0/bbl-v3.2.0_linux_x86-64
  chmod +x ./bbl-v3.2.0_linux_x86-64
popd

BBL_STATE_DIR=`pwd`/bbl-state
mkdir -p $BBL_STATE_DIR

docker run \
  -t -i \
  -v $BBL_STATE_DIR:'/bbl-state' \
  -v `realpath $BOSH_RELEASE_DIR`:/bosh-candidate-release \
  -v `realpath $BOSH_STEMCELL_DIR`:/bosh-candidate-stemcell \
  -v `realpath $BBL_CLI_DIR`:/bbl-cli \
  -v `realpath $BOSH_CLI_DIR`:/bosh-cli \
  -v ~/workspace/bosh-deployment:/bosh-deployment \
  -v $DIR/..:/dns-release bosh/main-ruby-go \
  bash -c 'source /dns-release/bbl.env; /dns-release/ci/tasks/bbl-up.sh'

fly -t production execute -x --privileged --config=./ci/tasks/test-acceptance-windows.yml --inputs-from=dns-release/test-acceptance-windows --input=dns-release=$DIR/../ --input=bbl-state=$BBL_STATE_DIR

docker run \
  -t -i \
  -v $BBL_STATE_DIR:'/bbl-state' \
  -v `realpath $BOSH_RELEASE_DIR`:/bosh-candidate-release \
  -v `realpath $BOSH_STEMCELL_DIR`:/bosh-candidate-stemcell \
  -v `realpath $BBL_CLI_DIR`:/bbl-cli \
  -v `realpath $BOSH_CLI_DIR`:/bosh-cli \
  -v ~/workspace/bosh-deployment:/bosh-deployment \
  -v $DIR/..:/dns-release bosh/main-ruby-go \
  bash -c 'source /dns-release/bbl.env; /dns-release/ci/tasks/bbl-destroy.sh'

exit 0
