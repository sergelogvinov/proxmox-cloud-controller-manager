# Verify images

We'll be employing [Cosing's](https://github.com/sigstore/cosign) keyless verifications to ensure that images were built in Github Actions.

## Verify Helm chart

We will verify the keyless signature using the Cosign protocol.

```shell
cosign verify ghcr.io/sergelogvinov/charts/proxmox-cloud-controller-manager:0.1.5 --certificate-identity https://github.com/sergelogvinov/proxmox-cloud-controller-manager/.github/workflows/release-charts.yaml@refs/heads/main --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Verify containers

We will verify the keyless signature using the Cosign protocol.

```shell
# Edge version
cosign verify ghcr.io/sergelogvinov/proxmox-cloud-controller-manager:edge --certificate-identity https://github.com/sergelogvinov/proxmox-cloud-controller-manager/.github/workflows/build-edge.yaml@refs/heads/main --certificate-oidc-issuer https://token.actions.githubusercontent.com

# Releases
cosign verify ghcr.io/sergelogvinov/proxmox-cloud-controller-manager:v0.2.0 --certificate-identity https://github.com/sergelogvinov/proxmox-cloud-controller-manager/.github/workflows/release.yaml@refs/heads/main --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
