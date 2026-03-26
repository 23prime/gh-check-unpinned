# gh-check-unpinned

This GitHub CLI extension detects the use of actions in the workflow of repositories under a specific owner (user or organization) that are not SHA-pinned.

## Install extension

```sh
gh extension install 23prime/gh-check-unpinned
```

## Usage

```sh
gh check-unpinned [--include-archived] [--include-forks] [--json] <owner>
```

### Arguments

| Argument | Description |
| --- | --- |
| `<owner>` | GitHub user or organization name (required) |

### Options

| Flag | Description |
| --- | --- |
| `--include-archived` | Include archived repositories (excluded by default) |
| `--include-forks` | Include forked repositories (excluded by default) |
| `--json` | Output findings as JSON instead of plain text |

### What is "SHA-pinned"?

An action reference is considered SHA-pinned when its `uses:` field specifies a full 40-character commit SHA, for example:

```yaml
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683
```

Tags (`@v4`) and branch names (`@main`) are *not* considered pinned because they are mutable and can be changed by the upstream repository at any time.

### Examples

#### Basic — check all non-archived, non-forked repos under an org

```sh
$ gh check-unpinned my-org
my-org/some-repo/.github/workflows/ci.yml: actions/checkout@v4
my-org/some-repo/.github/workflows/ci.yml: actions/setup-go@v5
my-org/another-repo/.github/workflows/release.yml: actions/upload-artifact@v3
```

When no unpinned actions are found, a success message is printed:

```sh
$ gh check-unpinned my-org
All actions are SHA-pinned.
```

#### Include archived and forked repositories

```sh
$ gh check-unpinned --include-archived --include-forks my-org
my-org/archived-repo/.github/workflows/build.yml: actions/cache@v3
```

#### JSON output

Use `--json` to get machine-readable output, for example to pipe into `jq`:

```sh
$ gh check-unpinned --json my-org | jq '.[] | select(.repo == "my-org/some-repo")'
{
  "repo": "my-org/some-repo",
  "workflow": ".github/workflows/ci.yml",
  "action": "actions/checkout@v4"
}
```

Each JSON object has the following fields:

| Field | Description |
| --- | --- |
| `repo` | Repository in `owner/name` format |
| `workflow` | Path to the workflow file (e.g., `.github/workflows/ci.yml`) |
| `action` | Unpinned action reference as written in the workflow |

## Development

### Pre requirements

- [mise](https://mise.jdx.dev)

### Get start development

1. Setup project.

    ```sh
    mise run setup
    ```

2. Run application.

   ```sh
   mise run go-run
   ```

3. Check project.

    ```sh
    mise run check
    ```
