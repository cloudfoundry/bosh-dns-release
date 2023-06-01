#!/bin/bash
main() {
  source $PWD/bosh-dns-release/ci/assets/utils.sh
  local output_dir="$PWD/updated-bbl-state/"
  local bbl_state_env_repo_dir=$PWD/bbl-state
  trap "commit_bbl_state_dir ${bbl_state_env_repo_dir} ${BBL_STATE_DIR} ${output_dir} 'Update bbl state dir'" EXIT

  export TEST_STRESS_ASSETS=$PWD/bosh-dns-release/ci/assets/test-stress
  export BOSH_DOCKER_CPI_RELEASE_TARBALL="$( echo $PWD/bosh-docker-cpi-release/*.tgz )"

  mkdir -p bbl-state/${BBL_STATE_DIR}

  pushd bbl-state/${BBL_STATE_DIR}
    bbl version
    bbl plan > bbl_plan.txt

    # Customize environment
    cp $TEST_STRESS_ASSETS/director/*.sh .

    bbl --debug up

    eval "$(bbl print-env)"

    # Need to delete the bosh-dns runtime config because bbl uses a hard-coded
    # bosh-deployment which specifies a bosh-dns version that may conflict with the
    # one we are trying to test.
    bosh delete-config --type=runtime --name=dns -n
  popd
}

main
