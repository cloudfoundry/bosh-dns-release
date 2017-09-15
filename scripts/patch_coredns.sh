#!/usr/bin/env bash

set -e

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

git apply $DIR/../patches/make_cache_usable_as_library.patch
