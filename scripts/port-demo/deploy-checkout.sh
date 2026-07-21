#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "Usage: $0 <full-commit-sha> [release-version]" >&2
  exit 2
fi

commit_sha=$1
release_version=${2:-revenue-risk-${commit_sha:0:8}}

if [[ ! $commit_sha =~ ^[0-9a-f]{40}$ ]]; then
  echo "The commit SHA must contain exactly 40 lowercase hexadecimal characters." >&2
  exit 2
fi

if [[ ! $release_version =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "The release version may contain only letters, numbers, dots, underscores, and hyphens." >&2
  exit 2
fi

if [[ $(kubectl config current-context) != *otel-demo-eks* ]]; then
  echo "kubectl is not connected to the otel-demo-eks cluster." >&2
  exit 1
fi

release_values=$(mktemp)
trap 'rm -f "$release_values"' EXIT

sed \
  -e "s/REPLACE_WITH_FULL_COMMIT_SHA/$commit_sha/g" \
  -e "s/REPLACE_WITH_RELEASE_VERSION/$release_version/g" \
  kubernetes/otel-demo/checkout-release-values.template.yaml \
  > "$release_values"

helm upgrade otel-demo \
  open-telemetry/opentelemetry-demo \
  --version 0.40.10 \
  --namespace otel-demo \
  --reuse-values \
  --values kubernetes/otel-demo/grafana-traces-values.yaml \
  --values "$release_values" \
  --wait \
  --timeout 15m

kubectl rollout status deployment/checkout --namespace otel-demo --timeout=5m
kubectl get deployment checkout \
  --namespace otel-demo \
  --output custom-columns='NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image,VERSION:.spec.template.metadata.annotations.resource\.opentelemetry\.io/service\.version'
