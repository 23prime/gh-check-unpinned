# gh-check-unpinned

This GitHub CLI extension detects the use of actions in the workflow of repositories under a specific owner (user or organization) that are not SHA-pinned.

## Install extension

```sh
gh extension install 23prime/gh-check-unpinned
```

## Usage

```sh
gh check-unpinned <owner>
```

- `<owner>` — GitHub user or organization name (required)

### Example

```sh
$ gh check-unpinned my-org
my-org/some-repo/.github/workflows/ci.yml: actions/checkout@v4
my-org/some-repo/.github/workflows/ci.yml: actions/setup-go@v5
my-org/another-repo/.github/workflows/release.yml: actions/upload-artifact@v3
```

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
