#!/usr/bin/env bash

charts_dir=charts

bin_dir=bin

release_dir="$(realpath release)"
release_helm_dir="$release_dir/helm"
release_docker_dir="$release_dir/docker"

version="$(tools/version.bash get)"

docker_repository="joshmeranda"

log()
{
  level="$1"
  shift
  echo "$(date) [$level] $*"
}

log_info()
{
  log "info" "$@"
}

log_warn()
{
  log "warn" "$@"
}

log_error()
{
  log "error" "$@"
}

package_binaries()
{
  local binaries=(kubedump kubedump-server)

  log_info 'packaging binaries'
  mkdir --parents "$release_dir"

  for bin in "${binaries[@]}"; do
    if ! make "$bin"; then
      log_error "could not compile binary '$bin'"
      return 1
    fi
  done

  if ! cp --recursive "$bin_dir" "$release_dir"; then
    log_error "could not copy bin dir '$bin_dir' into '$release_dir'"
    return 1
  fi
}

package_helm()
{
  log_info 'packaging helm charts'
  mkdir --parents "$release_helm_dir"

  for chart in $charts_dir/*; do
    log_info "packaging chart '$chart'"

    helm package --destination "$release_helm_dir" "$chart"
  done
}

package_docker()
{
  log_info "packaging docker images"
  mkdir --parents "$release_docker_dir"

  image_ref="$docker_repository/kubedump-server:$version"
  image_archive="$release_docker_dir/kubedump-server-$version.tar"

  log_info "creating docker image '$image_ref'"
  if ! make docker; then
    log_error "failed to build image '$image_ref'"
    return 1
  fi

  log_info "saving image 'image_ref'"
  if ! docker save "$image_ref" --output "$image_archive"; then
    log_error "failed to save image '$image_ref' to file '$image_archive'"
    return 1
  fi
}

package_binaries || exit 1
package_helm || exit 1
package_docker || exit 1
