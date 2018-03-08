#!/bin/sh
bosh delete-env \
  ${BBL_STATE_DIR}/director-manifest.yml \
  --state  ${BBL_STATE_DIR}/vars/bosh-state.json \
  --vars-store  ${BBL_STATE_DIR}/vars/director-vars-store.yml \
  -l  ${BBL_STATE_DIR}/vars/director-vars-file.yml
