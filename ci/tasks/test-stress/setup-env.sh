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
    cp $TEST_STRESS_ASSETS/terraform/* terraform/
    cp $TEST_STRESS_ASSETS/director/*.sh .

    bbl --debug up
  popd
}

main
