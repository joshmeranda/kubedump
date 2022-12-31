#!/usr/bin/env bash

charts_dir=charts
release_dir="$(realpath release)"

mkdir --parents "$release_dir"

cd "$charts_dir" || exit 1

for chart in *; do
  chart_version=$(yq eval '.version' "$chart/Chart.yaml")
  chart_name=$(yq eval '.name' "$chart/Chart.yaml")

  tar --gzip --create --file "$release_dir/$chart_name-$chart_version.tgz" "$chart"
done