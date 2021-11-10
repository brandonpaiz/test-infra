#!/bin/bash

check_env () {
  env_vars=("$@")
  missing_var=0
  for var in "${env_vars[@]}"; do
    if [ -z "${!var}" ]; then
      echo "${var} unset"
      missing_var=1
    fi
  done
  return $missing_var
}

substitute_env_in_files () {
  files=("$@")
  for file in "${files[@]}"; do
    tmp=$(mktemp)
    cp --attributes-only --preserve ${file} ${tmp}
    cat ${file} | envsubst > $tmp && mv $tmp $file
    #envsubst < ${file} > "${file}.cfg"
  done
}

deploy () {
  directories=("$@")
  for directory in "${directories[@]}"; do
    pushd ${directory} > /dev/null
    gcloud app deploy --project ${GCP_PROJECT_ID}
    popd > /dev/null
  done
}

check_env GCP_PROJECT_ID PG_USER PG_PASS PG_DATABASE CLOUD_SQL_INSTANCE || exit 1

substitute_env_in_files \
  "./grafana/app.yaml" \
  "./grafana/provisioning/datasources/postgres_config.yaml" \
  "./database_transfer/app.yaml" \
  "./database_transfer/config/transfer.yaml"

#deploy grafana database_transfer #TODO: once database_transfer shown to be working
deploy grafana

