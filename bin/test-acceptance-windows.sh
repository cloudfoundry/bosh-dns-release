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

# Comment this out if you are going to do this over and over and you
# don't want to download the files each time.
trap cleanup EXIT

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

fly -t production login

if [ ! -e bbl.env ]; then
    temp_env_file=$(mktemp bbl.env)
    chmod +x $temp_env_file
    cat <<EOF >$temp_env_file
export BBL_GCP_SERVICE_ACCOUNT_KEY='$(lpass show 3654688481222762882 --notes | gobosh int - --path /bbl_gcp_service_account_key_id)'
export BBL_GCP_PROJECT_ID=cf-bosh-core
export BBL_GCP_ZONE=us-central1-a
export BBL_GCP_REGION=us-central1
export BBL_IAAS=gcp
EOF
fi

# Download bosh-cli if it doesn't exist
if [ ! -d bosh-cli ]; then
    BOSH_CLI_DIR=$(mktemp -d bosh-cli)
else
    BOSH_CLI_DIR="${DIR}/../bosh-cli"
fi
if [ ! -e $BOSH_CLI_DIR/bosh-cli-linux ]; then
    pushd ${BOSH_CLI_DIR}
      curl -o bosh-cli-linux https://s3.amazonaws.com/bosh-cli-alpha-artifacts/alpha-bosh-cli-0.0.250-linux-amd64
    popd
fi

# Download bosh-release if it doesn't exist
if [ ! -d bosh-release ]; then
    BOSH_RELEASE_DIR=$(mktemp -d bosh-release)
else
    BOSH_RELEASE_DIR="${DIR}/../bosh-release"
fi
if [ ! -e ${BOSH_RELEASE_DIR}/bosh-dev-release.tgz ]; then
    pushd $BOSH_RELEASE_DIR
      # For latest 262.1, use: https://bosh.io/d/github.com/cloudfoundry/bosh?v=262.1
      curl -L -J -o bosh-dev-release.tgz https://s3.amazonaws.com/bosh-compiled-release-tarballs/bosh-261.4-ubuntu-trusty-3363.25-20170530-233054-649382379-20170530233108.tgz
    popd
fi

# Download the stemcell if it doesn't exist
if [ ! -d bosh-stemcell ]; then
    BOSH_STEMCELL_DIR=$(mktemp -d bosh-stemcell)
else
    BOSH_STEMCELL_DIR="${DIR}/../bosh-stemcell"
fi
if [ ! -e ${BOSH_STEMCELL_DIR}/bosh-stemcell-3362-warden-boshlite-ubuntu-trusty-go_agent.tgz ]; then
    pushd $BOSH_STEMCELL_DIR
      curl -L -J -o bosh-stemcell-3362-warden-boshlite-ubuntu-trusty-go_agent.tgz https://bosh.io/d/stemcells/bosh-warden-boshlite-ubuntu-trusty-go_agent?v=3363.22
    popd
fi

# Download the bbl cli if it doesn't exist
if [ ! -d bbl-cli ]; then
    BBL_CLI_DIR=$(mktemp -d bbl-cli)
else
    BBL_CLI_DIR="${DIR}/../bosh-cli"
fi
if [ ! -e ${BBL_CLI_DIR}/bbl-v3.2.0_linux_x86-64 ]; then
    pushd $BBL_CLI_DIR
      curl -o bbl-v3.2.0_linux_x86-64 -L -v https://github.com/cloudfoundry/bosh-bootloader/releases/download/v3.2.0/bbl-v3.2.0_linux_x86-64
      chmod +x ./bbl-v3.2.0_linux_x86-64
    popd
fi

BBL_STATE_DIR=`pwd`/bbl-state
mkdir -p $BBL_STATE_DIR

docker pull bosh/main-ruby-go

docker run \
  -t -i \
  -v $BBL_STATE_DIR:'/bbl-state' \
  -v `realpath $BOSH_RELEASE_DIR`:/bosh-candidate-release \
  -v `realpath $BOSH_STEMCELL_DIR`:/bosh-candidate-stemcell \
  -v `realpath $BBL_CLI_DIR`:/bbl-cli \
  -v `realpath $BOSH_CLI_DIR`:/bosh-cli \
  -v ~/workspace/bosh-deployment:/bosh-deployment \
  -v $DIR/..:/dns-release \
  bosh/main-ruby-go \
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

# Clean up creds.yml so that the next bbl-up will recreate certs for the new instances.
rm -f $BBL_STATE_DIR/creds.yml

exit 0
