# deeph-cli

NPM launcher for `deepH` (Go CLI).

It downloads the correct prebuilt binary for your OS from GitHub Releases and exposes:

```bash
deeph
```

## Install

```bash
npm i -g deeph-cli
deeph studio
```

## Runtime overrides

Optional environment variables:

- `DEEPH_RELEASE_TAG`: release tag to install (default: `v<package.version>`)
- `DEEPH_GITHUB_OWNER`: GitHub owner (default: `tom-lisboa`)
- `DEEPH_GITHUB_REPO`: GitHub repo (default: `deepH`)

## Supported targets

- macOS: `arm64`, `x64`
- Linux: `arm64`, `x64`
- Windows: `arm64`, `x64`
