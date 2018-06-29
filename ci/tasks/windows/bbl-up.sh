#!/bin/bash
main() {
  source $PWD/bosh-dns-release/ci/assets/utils.sh
  local output_dir="$PWD/envs-output/"
  local bbl_state_env_repo_dir=$PWD/envs
  trap "commit_bbl_state_dir ${bbl_state_env_repo_dir} ${ENV_NAME} ${output_dir} 'Update bbl state dir'" EXIT

  mkdir -p envs/${ENV_NAME}

  pushd envs/${ENV_NAME}
    bbl version
    bbl plan > bbl_plan.txt
    bbl --debug up
    bbl print-env > .envrc
    source .envrc
    cp "${JUMPBOX_PRIVATE_KEY}" bosh_jumpbox_private.key

    sed -i '/JUMPBOX_PRIVATE_KEY\|BOSH_ALL_PROXY\|CREDHUB_PROXY/d' .envrc

    echo "export JUMPBOX_ADDRESS=$(bbl jumpbox-address):22"
    echo 'export JUMPBOX_PRIVATE_KEY=$PWD/bosh_jumpbox_private.key' >> .envrc
    echo "export BOSH_ALL_PROXY=\"ssh+socks5://jumpbox@$(bbl jumpbox-address):22?private-key=\$JUMPBOX_PRIVATE_KEY\"" >> .envrc
    echo 'export CREDHUB_PROXY="${BOSH_ALL_PROXY}"' >> .envrc
  popd
}

main
