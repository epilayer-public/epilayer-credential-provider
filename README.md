# epilayer-credential-provider

A [kubelet image credential provider](https://kubernetes.io/docs/tasks/administer-cluster/kubelet-credential-provider/)
that reads a token from a file on the node and returns it as the password for
image pulls. Whatever keeps that file up to date (for example a DaemonSet
writing to a hostPath) controls which credentials kubelet uses; the provider
reads the file on every call, so a rotated token is used as soon as kubelet's
cache entry expires.

It has no dependencies beyond the Go standard library and implements
`credentialprovider.kubelet.k8s.io/v1`.

## Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `--token-file` | (required) | File holding the token. Surrounding whitespace is trimmed. |
| `--username` | (required) | Username returned with the token. |
| `--cache-duration` | `1m` | How long kubelet may cache the credentials. Keep it short so rotated tokens are picked up; `0s` disables caching. |
| `--version` | | Print the version and exit. |

The credentials are returned for the registry host of the requested image
(`cacheKeyType: Registry`). Limit which images the provider is called for with
`matchImages` in the kubelet configuration.

## Kubelet configuration

Install the binary into the directory passed to kubelet as
`--image-credential-provider-bin-dir`, named like the provider's `name` below,
and pass this file as `--image-credential-provider-config`:

```yaml
apiVersion: kubelet.config.k8s.io/v1
kind: CredentialProviderConfig
providers:
  - name: epilayer-credential-provider
    apiVersion: credentialprovider.kubelet.k8s.io/v1
    matchImages:
      - "registry.example.com/my-project/*"
    defaultCacheDuration: "1m"
    args:
      - --token-file=/var/lib/registry-credentials/token
      - --username=k8s
```

## Releases

Pushing a tag `vX.Y.Z` builds static binaries for `linux/amd64` and
`linux/arm64` and publishes them with a `SHA256SUMS` file as a GitHub release.
Verify a download with:

```sh
sha256sum --check --ignore-missing SHA256SUMS
```

## Development

```sh
go test ./...
go vet ./...
golangci-lint run
```

Or run all checks with `pre-commit run --all-files`.

## License

[Mozilla Public License 2.0](LICENSE)
