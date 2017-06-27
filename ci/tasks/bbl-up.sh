#!/bin/bash -eux

set -o pipefail

ROOT_DIR=$PWD
BBL_STATE_DIR=$ROOT_DIR/bbl-state

mkdir -p $BBL_STATE_DIR/bin
export PATH=$BBL_STATE_DIR/bin:$PATH

set +u
source /usr/local/share/chruby/chruby.sh
chruby ruby-2.3.1
set -u

apt-get install -y zip

wget https://releases.hashicorp.com/terraform/0.9.4/terraform_0.9.4_linux_amd64.zip
unzip terraform_0.9.4_linux_amd64.zip
mv terraform $BBL_STATE_DIR/bin/
chmod +x $BBL_STATE_DIR/bin/terraform

cp $(realpath $ROOT_DIR/bosh-cli/bosh-cli-*) $BBL_STATE_DIR/bin/bosh
chmod +x $BBL_STATE_DIR/bin/bosh

cp $(realpath $ROOT_DIR/bbl-cli/bbl-*_linux_x86-64) $BBL_STATE_DIR/bin/bbl
chmod +x $BBL_STATE_DIR/bin/bbl

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
