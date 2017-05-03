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

bosh -d bosh-dns-windows-acceptance -n deploy $ROOT_DIR/dns-release/ci/assets/windows-acceptance-manifest.yml \
    -v dns_release_path=$ROOT_DIR/dns-release

bosh -d bosh-dns-windows-acceptance run-errand acceptance-tests-windows
