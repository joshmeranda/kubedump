#!/usr/bin/env bash

log_file=/tmp/kubedump-verify.log
version="$(tools/version.bash get)"
docker_image="joshmeranda/kubedump-server:$version"

passed=0
failed=0

function check() {
  echo -n "$1:  "
  if ! eval "$2" &> "$log_file"; then
    echo "ERROR"
    ((failed++))
    cat "$log_file"
  else
    echo "OK"
    ((passed++))
  fi
}

function check_gofmt() {
  fmt_diff="$(find . -name \*.go -not -name '*_test.go' -exec gofmt -d '{}' +)"
  if [ -n  "$fmt_diff" ]; then
    echo "$fmt_diff"
    return 1
  fi
}

function check_app_version() {
  app_version=$(yq '.appVersion' charts/kubedump-server/Chart.yaml | tr --delete '"')

  if [ "$app_version" != "$version" ]; then
    echo "$app_version != $version"
    return 1
  fi
}

check 'GO TEST         ' 'make test'
check 'GO FMT          ' 'check_gofmt'
check 'HELM LINT       ' 'helm lint charts/kubedump-server'
check 'HELM APP VERSION' 'check_app_version'
check 'DOCKER IMAGE    ' "docker manifest inspect $docker_image"

echo -e "\nPASSED: $passed\tFAILED: $failed"