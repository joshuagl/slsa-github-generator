# Generation of SLSA3+ provenance for Golang projects

This document explains how to use the builder for Golang projects.

---

[Generation of provenance](#generation)

- [Supported triggers](#supported-triggers)
- [Configuration file](#configuration-file)
- [Migration from goreleaser](#migration-from-goreleaser)
- [Workflow inputs](#workflow-inputs)
- [Workflow Example](#workflow-example)
- [Example provenance](#example-provenance)
- [BuildConfig format](#buildconfig-format)

[Verification of provenance](#verification-of-provenance)

- [Installation](#installation)
- [Inputs](#inputs)
- [Command line examples](#command-line-examples)

---

## Generation

To generate provenance for a golang binary, follow the steps below:

### Supported triggers

Most GitHub trigger events are supported, at the exception of `pull_request`. We have extensively tested the 
following triggers: `schedule`, `push` (including new tags) and manual `workflow_dispatch`.

If you would like support for `pull_request`, please tell us about your use case and [file an issue](https://github.com/slsa-framework/slsa-github-generator/issues/new).

### Configuration file

Define a configuration file called `.slsa-goreleaser.yml` in the root of your project.

```yml
# Version for this file.
version: 1

# (Optional) List of env variables used during compilation.
env:
  - GO111MODULE=on
  - CGO_ENABLED=0

# (Optional) Flags for the compiler.
flags:
  - -trimpath
  - -tags=netgo

# The OS to compile for. `GOOS` env variable will be set to this value.
goos: linux 

# The architecture to compile for. `GOARCH` env variable will be set to this value.
goarch: amd64

# (Optional) Entrypoint to compile. (Optional)
# main: ./path/to/main.go

# (Optional) Working directory. (default: root of the project)
# dir: /path/to/dir

# Binary output name.
# {{ .Os }} will be replaced by goos field in the config file.
# {{ .Arch }} will be replaced by goarch field in the config file.
binary: binary-{{ .Os }}-{{ .Arch }}

# (Optional) ldflags generated dynamically in the workflow, and set as the `env` input variables in the workflow.
ldflags:
  - "{{ .Env.VERSION_LDFLAGS }}"
```

### Migration from goreleaser

If you are already using Goreleaser, you may be able to migrate to our builder using multiple config files for each build. However, this is cumbersome and we are working on supporting multiple builds in a single config file for future releases. 

In the meantime, you can use both Goreleaser and this builder in the same repository. For example, you can pick one build you would like to start generating provenance for. Goreleaser and this builder can co-exist without interfering with one another, so long as they build fr different OS/Arch. We think gradual adoption is good for project to get used to SLSA.

The configuration file accepts many of the common fields Goreleaser uses, as you can see in the [example](#configuration-file). The configuration file also supports two variables: `{{ .Os }}` and `{{ .Arch }}`. If you need suppport for other variables, please [open an issue](https://github.com/slsa-framework/slsa-github-generator/issues/new).

### Workflow inputs

The builder workflow [slsa-framework/slsa-github-generator/.github/workflows/builder_go_slsa3.yml](.github/workflows/builder_go_slsa3.yml) accepts the following inputs:

| Name         | Required | Description    | Default                                                                                                                                                                                                                                           |
| ------------------ | -------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `config-file` | no      | `.github/workflows/slsa-goreleaser.yml` | The configuration file for the builder. A path within the calling repository. |
| `evaluated-envs`        | no       | empty value | A list of environment variables, seperated by `,`: `VAR1: value, VAR2: value`. This is typically used to pass dynamically-generated values, such as `ldflags`. Note that only environment variables with names starting with `CGO_` or `GO` are accepted. |
| `go-version` | yes      | The go version for your project. This value is passed, unchanged, to the [actions/setup-go](https://github.com/actions/setup-go) action when setting up the environment |
| `upload-assets` | no    | true on new tags | Whether to upload assets to a GitHub release or not. |

### Workflow Example

Create a new workflow, say `.github/workflows/slsa-goreleaser.yml`.

Make sure that you reference the trusted builder with a semnatic version of the form `vX.Y.Z`. The build will fail
if you reference it via a shorter tag like `vX.Y` or `vX`. 

Refencing via hash is currently not supported due to limitations
of the reusable workflow APIs. (We are working with GitHub to address this limitation).

```yaml
name: SLSA go releaser
on:
  workflow_dispatch:
  push:
    tags:
      - "*"

permissions: read-all

jobs:
  # Generate ldflags dynamically.
  # Optional: only needed for ldflags.
  args:
    runs-on: ubuntu-latest
    outputs:
      ldflags: ${{ steps.ldflags.outputs.value }}
    steps:
      - id: checkout
        uses: actions/checkout@ec3a7ce113134d7a93b817d10a8272cb61118579 # v2.3.4
        with:
          fetch-depth: 0
      - id: ldflags
        run: |
          echo "::set-output name=value::$(./scripts/version-ldflags)"

  # Trusted builder.
  build:
    permissions:
      id-token: write
      contents: write
      actions: read
    needs: args
    uses: slsa-framework/slsa-github-generator/.github/workflows/builder_go_slsa3.yml@v1.0.0
    with:
      go-version: 1.17
      # Optional: only needed if using ldflags.
      evaluated-envs: "VERSION_LDFLAGS:${{needs.args.outputs.ldflags}}"
```

### Example provenance

An example of the provenance generated from this repo is below:

```json
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "https://slsa.dev/provenance/v0.2",
  "subject": [
    {
      "name": "binary-linux-amd64",
      "digest": {
        "sha256": "7bf2e6ebb97e1bdb669d9df73048247f141e2f8e72ab59f23d456f1bc5a041dc"
      }
    }
  ],
  "predicate": {
    "builder": {
      "id": "https://github.com/slsa-framework/slsa-github-generator/.github/workflows/builder_go_slsa3.yml@v1.0.0"
    },
    "buildType": "https://github.com/slsa-framework/slsa-github-generator/go@v1",
    "invocation": {
      "configSource": {
        "uri": "git+https://github.com/ianlewis/actions-test@refs/heads/main",
        "digest": {
          "sha1": "d29d1701b47bbbe489e94b053611e5a7bf6d9414"
        },
        "entryPoint": ".github/workflows/release.yml"
      },
      "parameters": {},
      "environment": {
        "github_actor": "ianlewis",
        "github_actor_id": "123456",
        "github_base_ref": "",
        "github_event_name": "workflow_dispatch",
        "github_event_payload": ...,
        "github_head_ref": "",
        "github_ref": "refs/heads/main",
        "github_ref_type": "branch",
        "github_repository_id": "8923542",
        "github_repository_owner": "ianlewis",
        "github_repository_owner_id": "123456",
        "github_run_attempt": "1",
        "github_run_id": "2193104371",
        "github_run_number": "16",
        "github_sha1": "d29d1701b47bbbe489e94b053611e5a7bf6d9414"
      }
    },
    "buildConfig": {
      "version": 1,
      "steps": [
        {
          "command": [
            "/opt/hostedtoolcache/go/1.17.10/x64/bin/go",
            "mod",
            "vendor"
          ],
          "env": null,
          "workingDir": "/home/runner/work/ianlewis/actions-test"
        },
        {
          "command": [
            "/opt/hostedtoolcache/go/1.17.10/x64/bin/go",
            "build",
            "-mod=vendor",
            "-trimpath",
            "-tags=netgo",
            "-o",
            "binary-linux-amd64-config1"
          ],
          "env": [
            "GOOS=linux",
            "GOARCH=amd64",
            "GO111MODULE=on",
            "CGO_ENABLED=0"
          ],
          "workingDir": "/home/runner/work/ianlewis/actions-test"
        }
      ]

    },
    "metadata": {
      "completeness": {
        "parameters": true,
        "environment": false,
        "materials": false
      },
      "reproducible": false
    },
    "materials": [
      {
        "uri": "git+https://github.com/ianlewis/actions-test@refs/heads/main",
        "digest": {
          "sha1": "d29d1701b47bbbe489e94b053611e5a7bf6d9414"
        }
      }
    ]
  }
}
```

### BuildConfig format

The `BuildConfig` contains the following fields:

`version`: The version of the `BuildConfig` format.

`steps`: The steps that were performed in the buid.

`steps[*].command`: The list of commands that were executed in a step.

```json
  "command": [
    "/opt/hostedtoolcache/go/1.17.10/x64/bin/go",
    "mod",
    "vendor"
  ],
```

`steps[*].env`: Any environment variables used in the command, including any OS environment variables and those set in the configuration file.

```json
  "env": [
    "GOOS=linux",
    "GOARCH=amd64",
    "GO111MODULE=on",
    "CGO_ENABLED=0"
  ],
```

`steps[*].workingDir`: The working directory where the steps were performed in the runner.

```json
  "workingDir": "/home/runner/work/ianlewis/actions-test"
```

## Verification of provenance

To verify the provenance, use the [github.com/slsa-framework/slsa-verifier](https://github.com/slsa-framework/slsa-verifier) project.

### Installation

To install the verifier, see [slsa-framework/slsa-verifier#installation](https://github.com/slsa-framework/slsa-verifier#installation).

### Inputs

The inputs of the verifier are described in [slsa-framework/slsa-verifier#available-options](https://github.com/slsa-framework/slsa-verifier#available-options).

### Command line examples

A command line example is provided in [slsa-framework/slsa-verifier#example](https://github.com/slsa-framework/slsa-verifier#example).
