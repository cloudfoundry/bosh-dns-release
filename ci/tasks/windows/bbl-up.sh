#!/bin/bash
main() {
  source $PWD/bosh-dns-release/ci/assets/utils.sh
  local output_dir="$PWD/envs-output/"
  local bbl_state_env_repo_dir=$PWD/envs
  trap "commit_bbl_state_dir ${bbl_state_env_repo_dir} ${ENV_NAME} ${output_dir} 'Update bbl state dir'" EXIT

  mkdir -p envs/${ENV_NAME}
  local bosh_release_path=$(echo $PWD/bosh-candidate-release/*.tgz)

  pushd envs/${ENV_NAME}
    bbl version
    bbl plan > bbl_plan.txt
    # Use the local bosh release
    sed -i "/bosh create-env/a -o \${BBL_STATE_DIR}/bosh-deployment/local-bosh-release-tarball.yml -v local_bosh_release=${bosh_release_path} \\\\" create-director.sh
    # Remove the iam profile ops file - doesn't work with our assume role setup
    sed -i "/iam-instance-profile/d" create-director.sh
    bbl --debug up
    bbl print-env > .envrc
    source .envrc
    cp "${JUMPBOX_PRIVATE_KEY}" bosh_jumpbox_private.key

    sed -i '/JUMPBOX_PRIVATE_KEY\|BOSH_ALL_PROXY\|CREDHUB_PROXY/d' .envrc

    echo "export JUMPBOX_ADDRESS=$(bbl jumpbox-address):22" >> .envrc
    echo 'export JUMPBOX_PRIVATE_KEY=$PWD/bosh_jumpbox_private.key' >> .envrc
    echo "export BOSH_ALL_PROXY=\"ssh+socks5://jumpbox@$(bbl jumpbox-address):22?private-key=\$JUMPBOX_PRIVATE_KEY\"" >> .envrc
    echo 'export CREDHUB_PROXY="${BOSH_ALL_PROXY}"' >> .envrc
  popd
}

main
