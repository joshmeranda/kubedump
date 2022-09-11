#!/usr/bin/env bash

usage="Usage: $(basename $0) <CMD>

Commands:
  get           get the current git numbers tag
  set <tag>     create a new numbers tag for the current commit
  bump <major|minor|patch|rc [release]>
                bump the numbers number and create a new tag at the
                current commit
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
    local most_recent_tag="$(git tag --list | tail --lines 1)"
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

  echo 'don'\'t' forget to make these steps:'
  echo '  1) Update chart appVersion'
  echo '  2) Push new docker image'
}

function bump() {
  if [ "$#" -lt 1 ]; then
    echo -e "expected one of major, minor, patch, or rc but found none\n$usage"
  fi

  local raw_version="$(get_version)"

  local rc="$(cut --delimiter '-' --fields 2 <<< "$raw_version")"
  local numbers="$(cut --delimiter '-' --fields 1 <<< "$raw_version")"

  IFS=. read -r major minor patch <<< "$numbers"

  case "$1" in
    major)
      ((major++))
      ;;
    minor)
      ((minor++))
      ;;
    patch)
      ((patch++))
      ;;
    rc)
      if [ "$2" == release ]; then
        rc=
      else
          if [ -z "$rc" ]; then
            rc="rc0"
          else
            rcno="$(cut -c3- <<< $rc)"
            ((rcno++))

            rc="rc$rcno"
          fi
        fi
      ;;
    *)
      echo -e "expected on of major, minor, or patch but found '$1'\n$usage"
      ;;
  esac

  local bumped
  if [ -z "$rc" ]; then
    local bumped="$major.$minor.$patch"
  else
    local bumped="$major.$minor.$patch-$rc"
  fi

  set_version "$bumped"
}

case "$1" in
  get)
    get_version
    ;;
  set)
    shift
    set_version "$@"
    ;;
  bump)
    shift
    bump "$@"
    ;;
  *)
    echo -e "received unknown command '$1'\n$usage"
    exit 3
    ;;
esac
