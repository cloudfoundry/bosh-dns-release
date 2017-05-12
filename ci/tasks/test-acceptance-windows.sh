#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD

set +u
source /usr/local/share/chruby/chruby.sh
chruby ruby-2.3.1
set -u

apt-get install -y zip
wget https://releases.hashicorp.com/terraform/0.9.4/terraform_0.9.4_linux_amd64.zip
unzip terraform_0.9.4_linux_amd64.zip
mv terraform /usr/local/bin/
chmod +x /usr/local/bin/terraform

mv $(realpath $ROOT_DIR/bosh-cli/bosh-cli-*) /usr/local/bin/bosh
chmod +x /usr/local/bin/bosh

mv $(realpath $ROOT_DIR/bbl-cli/bbl-*_linux_x86-64) /usr/local/bin/bbl
chmod +x /usr/local/bin/bbl

bbl up --no-director

function cleanup() {
  set +e
  bosh -d bosh-dns-windows-acceptance -n delete-deployment
  bosh -d bosh-dns -n delete-deployment
  bosh -d bosh-dns-shared-acceptance -n delete-deployment
  bosh -n delete-env $ROOT_DIR/bosh-manifest.yml \
    --vars-store $ROOT_DIR/creds.yml  \
    --state $ROOT_DIR/state.json \
    -l <(bbl bosh-deployment-vars)
  bbl destroy --no-confirm --skip-if-missing
  set -e
}

trap cleanup EXIT

bosh int $ROOT_DIR/bosh-deployment/bosh.yml \
  --vars-store $ROOT_DIR/creds.yml  \
  -l <(bbl bosh-deployment-vars) \
  -o $ROOT_DIR/bosh-deployment/local-bosh-release.yml \
  -o $ROOT_DIR/bosh-deployment/local-dns.yml \
  -o $ROOT_DIR/bosh-deployment/gcp/cpi.yml  \
  -o $ROOT_DIR/bosh-deployment/external-ip-not-recommended.yml \
  -v local_bosh_release=$ROOT_DIR/bosh-candidate-release/bosh-dev-release.tgz \
  > $ROOT_DIR/bosh-manifest.yml

bosh create-env $ROOT_DIR/bosh-manifest.yml \
  --state $ROOT_DIR/state.json \
  --vars-store $ROOT_DIR/creds.yml \
  -l <(bbl bosh-deployment-vars)

bosh int $ROOT_DIR/creds.yml --path /director_ssl/ca > ca.crt

export BOSH_CLIENT=admin
export BOSH_CLIENT_SECRET=$(bosh int $ROOT_DIR/creds.yml --path /admin_password)
export BOSH_ENVIRONMENT=$(bbl director-address)
export BOSH_CA_CERT=$ROOT_DIR/ca.crt

bosh -n update-cloud-config <(bbl cloud-config)
bosh -n upload-stemcell $ROOT_DIR/bosh-candidate-stemcell-windows/*.tgz

# Deploy and run tests that check address alias on windows locally
bosh -d bosh-dns-windows-acceptance -n deploy $ROOT_DIR/dns-release/ci/assets/windows-acceptance-manifest.yml \
    -v dns_release_path=$ROOT_DIR/dns-release

bosh -d bosh-dns-windows-acceptance run-errand acceptance-tests-windows

# Deploy and run tests that check the dns resolver on windows locally
bosh -d bosh-dns-windows-acceptance -n deploy --recreate $ROOT_DIR/dns-release/ci/assets/windows-acceptance-manifest.yml \
    -o $ROOT_DIR/dns-release/src/acceptance_tests/windows/disable_nameserver_override/manifest-ops.yml \
    -v dns_release_path=$ROOT_DIR/dns-release

bosh -d bosh-dns-windows-acceptance run-errand acceptance-tests-windows

# Deploy and run tests that check windows can serve DNS
bosh -d bosh-dns -n deploy $ROOT_DIR/dns-release/ci/assets/dns-windows.yml \
    -v dns_release_path=$ROOT_DIR/dns-release \
    -v acceptance_release_path=$ROOT_DIR/dns-release/src/acceptance_tests/dns-acceptance-release

export BOSH_DEPLOYMENT=bosh-dns

bosh -n upload-stemcell $ROOT_DIR/gcp-linux-stemcell/*.tgz

bosh -d bosh-dns-shared-acceptance -n deploy $ROOT_DIR/dns-release/ci/assets/shared-acceptance-manifest.yml \
    -v dns_release_path=$ROOT_DIR/dns-release \
    --var-file bosh_ca_cert=$BOSH_CA_CERT \
    -v bosh_client_secret=$BOSH_CLIENT_SECRET \
    -v bosh_client=$BOSH_CLIENT \
    -v bosh_environment=$BOSH_ENVIRONMENT \
    -v bosh_deployment=$BOSH_DEPLOYMENT

bosh -d bosh-dns-shared-acceptance run-errand acceptance-tests
