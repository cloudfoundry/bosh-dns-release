#!/bin/bash

set -e -o pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

fly -t production login

fly -t production execute -x --privileged \
  --config=./ci/tasks/test-acceptance.yml \
  --inputs-from=dns-release/test-acceptance \
  --input=dns-release=$DIR/../
