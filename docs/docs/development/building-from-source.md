# Building from Source

## Prerequisites

You'll need **Go**, **Node**, and **npm**. The required versions are defined in
`sensor_hub/go.mod` and `sensor_hub/ui/sensor_hub_ui/package.json`.

## Build

From the repo root:

```bash
git clone https://github.com/tom-molotnikoff/sensor-hub.git
cd sensor-hub/sensor_hub
```

Then build the UI and the Go binary:

```bash
cd ui/sensor_hub_ui && npm ci && npm run build && cd ../..
mkdir -p web/dist && cp -r ui/sensor_hub_ui/dist/* web/dist/
go build -o sensor-hub .
```

The `sensor-hub` binary embeds the UI static assets from `web/dist/`.

## Run Locally

```bash
./sensor-hub --config-dir=configuration
```

The binary serves on **port 8080** by default.

## OpenAPI Code Generation

The generated Go and TypeScript files are committed to the repository. You only
need to regenerate them if you modify the API spec (`api/openapi.yaml`).

**Go** (requires [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen)):

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# From sensor_hub/
go generate ./gen/...
```

**TypeScript** (from `sensor_hub/ui/sensor_hub_ui/`):

```bash
npm run generate:api
```

## Dev Mode

**Backend only** — skip the full UI build with a stub:

```bash
mkdir -p web/dist
echo '<!doctype html><html><body>stub</body></html>' > web/dist/index.html
go build -o sensor-hub .
```

**Frontend** — run the Vite dev server with hot module replacement:

```bash
cd ui/sensor_hub_ui
npm install
npm run dev
```

The Vite dev server starts on port 5173. See [Docker Dev Environment](docker-dev-environment.md) for the required environment variables.

## Building Packages

To build installable RPM or DEB packages locally, use the package build script.
This uses [nfpm](https://nfpm.goreleaser.com/) to produce packages identical in
structure to the CI-built releases (minus GPG signing).

### Prerequisites

| Tool | Required For     | Install                                                       |
|------|------------------|---------------------------------------------------------------|
| Go   | All builds       | [go.dev/dl](https://go.dev/dl/)                               |
| nfpm | All builds       | `go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest`    |
| npm  | Server builds    | [nodejs.org](https://nodejs.org/)                              |

### Usage

```bash
scripts/build-packages.sh <target> [options]
```

**Targets:**

| Target   | Description                                  |
|----------|----------------------------------------------|
| `cli`    | CLI-only package (no UI, no server config)   |
| `server` | Full server package (includes React UI)      |
| `all`    | Build both packages                          |

**Options:**

| Option             | Default            | Description                    |
|--------------------|--------------------|--------------------------------|
| `--arch <arch>`    | Host architecture  | `amd64` or `arm64`            |
| `--format <fmt>`   | Host-appropriate   | `rpm` or `deb`                |
| `--version <ver>`  | Auto from git tag  | Package version                |
| `--output-dir <d>` | `dist/`            | Output directory               |

### Examples

```bash
# Build CLI RPM for your machine (auto-detects everything)
scripts/build-packages.sh cli

# Build server DEB for arm64 with a specific version
scripts/build-packages.sh server --arch arm64 --format deb --version 1.2.0

# Build both packages
scripts/build-packages.sh all
```

### Version Auto-Detection

When `--version` is not specified, the script reads the latest git tag, bumps
the patch number, and appends `~dev1`. For example, if the latest tag is
`v1.1.1`, the generated version is `1.1.2~dev1`. The tilde suffix ensures the
dev version sorts above the current release but below the next release in both
RPM and DEB package managers.

### Output

Packages are written to `dist/` (or the directory specified by `--output-dir`):

```
dist/sensor-hub-cli-1.1.2~dev1-1.x86_64.rpm
dist/sensor-hub-1.1.2~dev1-1.x86_64.rpm
```

Install locally with:

```bash
# Fedora / RHEL
sudo dnf install dist/sensor-hub-cli-*.rpm

# Debian / Ubuntu
sudo apt install ./dist/sensor-hub-cli_*.deb
```
