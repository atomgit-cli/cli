# System Tests

`tests/system/` contains real GitCode CLI command tests.

All repository targets, including read and write tests, must be under
`infra-test/*`. The runner rejects any other repository before running cases.

## Testscript Suite

The primary suite is a Go `testscript` runner, similar to GitHub CLI's
acceptance-test style. It is guarded by the `system` build tag, so normal
`go test ./...` does not run remote tests.

Read-only system tests:

```bash
go test -tags=system ./tests/system
# or
make system-test
```

Write-path system tests are opt-in:

```bash
GC_SYSTEM_WRITE=1 go test -tags=system ./tests/system -run TestWriteScripts
# or
make system-test-write
```

Set `GC_SYSTEM_ASSIGNEE` to an assignable username to include the real
`issue create --assignee` and `issue edit --assignee` lifecycle test:

```bash
GC_SYSTEM_WRITE=1 GC_SYSTEM_ASSIGNEE=<username> go test -tags=system ./tests/system -run TestWriteScripts
```

Authenticate with the local `gc auth login` configuration before running the
suite. Real `GC_TOKEN` and `GITCODE_TOKEN` values are intentionally not copied
into the testscript environment because verbose testscript output includes its
environment.

Script cases live under:

- `tests/system/testdata/read/*.txtar`
- `tests/system/testdata/write/*.txtar`

Custom testscript commands:

- `require-infra <repo>`: fail unless the repository is `infra-test/*`.
- `json-ok <file>`: assert that a file, usually `stdout`, is valid JSON.
- `json-assert <file> <path> <type>`: assert JSON field presence and type.
- `json-value <file> <path> <expected>`: assert a JSON field's string value.
- `stdout2env <name> <regexp>`: capture one stdout regexp group into an env var.
- `defer-close-issue <number>`: close a created write-test issue during cleanup.
- `defer-close-issue-title <title>`: close matching open issues during cleanup.
- `defer-delete-label <name>`: delete a created write-test label during cleanup.
- `unique-name <name> <prefix> [max-length]`: create a process/test scoped resource name;
  when set, the maximum length truncates only the readable prefix and preserves
  the complete PID and random suffix.

Supported `json-assert` types are `present`, `string`, `nonempty-string`,
`number`, `bool`, `object`, `array`, and `null`. Paths support object keys and
array indexes, for example `number`, `[0].title`, or `user.login`.

## Read-Only Suite

The legacy shell runner remains available for direct command-line smoke runs:

```bash
tests/system/run.sh --read
```

This builds `./gc` and runs read-only, dry-run, JSON contract, authentication
guard, and error-code cases against `infra-test/gctest1` by default.

Use a different infra-test repository:

```bash
tests/system/run.sh --read --repo infra-test/another-repo
```

## Write Suite

Write tests are opt-in and must also target `infra-test/*`.

```bash
tests/system/run.sh --write --write-repo infra-test/gctest1
```

The issue write case creates an issue and closes it during cleanup. The PR write
case is skipped unless an existing test branch is provided:

```bash
GC_SYSTEM_PR_HEAD=test-branch tests/system/run.sh --write --write-repo infra-test/gctest1
```

## Safety Rules

- Do not use personal repositories.
- Do not use production repositories.
- Do not use `atomgit-cli/cli`.
- Do not print, read, or pipe real tokens.
- Keep write cases self-cleaning whenever the remote API allows it.
