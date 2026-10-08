# Git Auto Sync repair checkout

This checkout maintains the installed GitJournal daemon. Preserve upstream sync,
Git hooks, repository configuration and publication behavior. Use native Git for
status and ignore decisions; filesystem watches must prune Git metadata and
ignored runtime directories. Watcher failures must terminate the process so the
existing service restart/watchdog policy can recover.

Run from the external build worktree with Go on PATH and Go caches/TMPDIR outside
the vault:

```sh
go test -p 1 ./... -count=1
go build -p 1 -o /tmp/git-auto-sync-daemon.test ./daemon
python3 tests/integration_daemon.py --daemon /tmp/git-auto-sync-daemon.test
```

Integration tests isolate HOME/configuration and use only throwaway repositories
and bare remotes. Never point a test at a live vault or tracker. Source, builds,
logs and toolchains stay outside the vault. Preserve the old binary before an
atomic replacement and restart only the owning user service. Do not push this
local deployment patch to the public upstream without explicit authorization.
