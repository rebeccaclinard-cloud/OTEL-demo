# Checkout release correlation

The Port demo checkout image is published as:

```text
ghcr.io/rebeccaclinard-cloud/otel-demo:<full-commit-sha>-checkout
```

The immutable commit tag and OCI image labels provide source provenance. The
Helm override adds the release version, commit SHA, repository URL, and
deployment environment as OpenTelemetry resource annotations on the checkout
Pod. The Demo Collector's Kubernetes attributes processor adds those values to
the checkout service's traces, metrics, and logs.

## Publish an image

1. Push `feature/revenue-at-risk` to the personal repository.
2. Open **Actions → Build Port demo checkout image**.
3. Run the workflow if it was not started automatically by the push.
4. Copy the complete commit SHA from the workflow run.
5. Make the resulting GHCR package public, or configure an EKS image pull
   secret before deployment.

The workflow uses GitHub's short-lived repository token. No registry password
or personal access token is stored in the repository.

## Deploy the immutable image

Confirm that `kubectl` is connected to `otel-demo-eks`, then run:

```bash
scripts/port-demo/deploy-checkout.sh FULL_COMMIT_SHA revenue-risk-v1
```

The script validates its inputs, preserves the existing Helm release values and
Grafana trace exporter, deploys the exact checkout image, waits for the rollout,
and prints the deployed image and release version.

## Verify release attributes

In Grafana, find a checkout trace and confirm these resource attributes:

```text
deployment.environment.name = eks-demo
service.version = revenue-risk-v1
vcs.ref.head.revision = <full commit SHA>
vcs.repository.url.full = https://github.com/rebeccaclinard-cloud/OTEL-demo
```

The same resource dimensions accompany the checkout business metrics, including
`business.checkout.revenue_at_risk`, so performance and business impact can be
grouped by release and correlated back to the owning repository and commit.
