#!/usr/bin/env bash

usage="Usage: $(basename $0) [-h] <CMD>

args:
  -h,--help     show this help text

Commands:
  get           get the current git numbers tag
  set <tag>     create a new numbers tag for the current commit
"

if [ "$#" -lt 1 ]; then
  echo -e "expected a command but found none\n$usage"
  exit
fi

function get_version() {
  local head_tag="$(git tag --contains HEAD)"

  if [ -n "$head_tag" ]; then
    echo "$head_tag"
  else
    local most_recent_tag="$(git tag --sort version:refname --list | tail --lines 1)"
    local commits_since_last="$(git log --oneline $most_recent_tag..HEAD | wc --lines)"

    echo "$most_recent_tag-dev-$commits_since_last"
  fi
}

function set_version() {
  if [ "$#" -lt 1 ]; then
    echo -e "expected a tag value but found none\n$usage"
    exit 4
  fi

  git tag "$1"

  echo 'don'\'t' forget to:
  1) Update chart appVersion
  2) Push new docker image
  3) Push the current git tag'
}

case "$1" in
  -h|--help)
    echo "$usage"
    exit
    ;;
  get)
    get_version
    ;;
  set)
    shift
    set_version "$@"
    ;;
  *)
    echo -e "received unknown command '$1'\n$usage"
    exit 3
    ;;
esac
