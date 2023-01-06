#!/usr/bin/env bash

usage="Usage: $(basename "$0") [-h] [-s <check>]

args:
  -h,--help           show this help text
  -s,--skip <check>   skip this check (whitespace will be removed from given check)
"

log_file=/tmp/kubedump-verify.log
version="$(tools/version.bash get)"

passed=0
failed=0

skipped=()

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)
      echo "$usage"
      exit
      ;;
    -s|--skip)
      skipped=("$2")
      shift
      ;;
  esac

  shift
done

function should_skip() {
  local check_name="$(echo "$1" | xargs)"

  for i in "${skipped[@]}"; do
    if [ "$i" == "$check_name" ]; then
      return
    fi
  done

  return 1
}

function check() {
  echo -n "$1:  "

  if should_skip "$1"; then
    echo "SKIPPED"
  elif ! eval "$2" &> "$log_file"; then
    echo "ERROR"
    ((failed++))

    echo
    cat "$log_file"
    echo
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

function check_helm_version() {
  app_version=$(yq eval '.version' charts/kubedump-server/Chart.yaml | tr --delete '"')

  if [ "$app_version" != "$version" ]; then
    echo "$app_version != $version"
    return 1
  fi
}

function check_git_clean() {
  s="$(git status --porcelain)"

  if [ -n "$s" ]; then
    echo 'workspace not clean:'
    echo "$s"
    return 1
  fi
}

function check_remote_tag() {
  if ! grep --quiet "$version" <<< "$(git ls-remote --tags origin)"; then
    echo "no remote tag '$version' was found"
  fi
}

function check_docker_images() {
  images=("joshmeranda/kubedump-server:$version")

  for image in "${images[@]}"; do
    docker manifest inspect "$image" || return 1
  done
}

check 'CLEAN WORKSPACE          ' 'check_git_clean'
check 'GO BUILD KUBEDUMP        ' 'make kubedump'
check 'GO BUILD KUBEDUMP-SERVER ' 'make kubedump-server'
check 'GO UNIT TEST             ' 'make unit'
check 'GO INTEGRATION TEST      ' 'make integration'
check 'GO FMT                   ' 'check_gofmt'
check 'HELM LINT                ' 'helm lint charts/kubedump-server'
check 'HELM VERSION             ' 'check_helm_version'
check 'DOCKER IMAGE             ' 'check_docker_images'
check 'REMOTE GIT VERSION TAG   ' 'check_remote_tag'

echo -e "\nPASSED: $passed\tFAILED: $failed"

if [ "$failed" -ne 0 ]; then
  exit 1
fi