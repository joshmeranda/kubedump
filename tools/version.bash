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
    local current_commit="$(git rev-parse HEAD)"

    if [ -z "$(git status --porcelain)" ]; then
      echo "$most_recent_tag-$current_commit"
    else
      echo "$most_recent_tag-$current_commit-dirty  "
    fi

  fi
}

function set_version() {
  if [ "$#" -lt 1 ]; then
    echo -e "expected a tag value but found none\n$usage"
    exit 4
  fi

  git tag "$1"

  echo 'don'\'t' forget to:
  1) Push the current git tag'
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
