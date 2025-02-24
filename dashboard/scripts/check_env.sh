#!/bin/bash

# check_env checks that all required environment variables are set
check_env () {
  env_vars=("$@")
  missing_var=0
  for var in "${env_vars[@]}"; do
    if [ -z "${!var}" ]; then
      echo "${var} not set"
      missing_var=1
    fi
  done
  return $missing_var
}

check_env "$@"
