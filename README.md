# Watchtower, with per-container schedules

This is a fork of [Watchtower](https://github.com/nicholas-fedor/watchtower),
which automatically updates running Docker containers when their images change.
It adds two features, each optional and off by default:

- **Container schedules:** a container can set its own update schedule, so you
  can check some containers every few minutes and leave the rest on a daily
  schedule, all with one Watchtower.
- **Watch events:** Watchtower follows Docker's container events, so a
  container you create or recreate is checked straight away, and a new
  container's schedule label takes effect at once.

Everything else is upstream Watchtower, unchanged, and its
[documentation](https://watchtower.nickfedor.com/) applies in full.

## Thanks

All of the credit for Watchtower belongs to the people who built it and keep it
going. This fork adds a few hundred lines to their work.

- **[Nicholas Fedor](https://github.com/nicholas-fedor)**, who maintains
  [nicholas-fedor/watchtower](https://github.com/nicholas-fedor/watchtower), the
  actively developed Watchtower this fork is built on. His care over the
  codebase (its tests, its docs, its steady releases) is what made adding this
  feature straightforward.
- **The maintainers and contributors of
  [containrrr/watchtower](https://github.com/containrrr/watchtower)**, who
  carried Watchtower for years before it was archived.
- **Watchtower's original authors at CenturyLink Labs**, where it began. Its
  labels still carry the name: `com.centurylinklabs.watchtower.*`.
- **[Everyone listed as a contributor upstream](https://github.com/nicholas-fedor/watchtower#contributors).**

If this feature is useful to you, please star and support
[the upstream project](https://github.com/nicholas-fedor/watchtower). The goal
is for this fork not to need to exist: both features are being offered
upstream, for [issue #1589](https://github.com/nicholas-fedor/watchtower/issues/1589).

## What it adds

Upstream Watchtower checks every container on one schedule (`--schedule` or
`--interval`). Some containers deserve more attention than others: an app you
deploy several times a day versus a database you'd rather leave alone. Until
now that meant running a second Watchtower with its own scope.

With this fork:

- Turn on `--container-schedules` (or `WATCHTOWER_CONTAINER_SCHEDULES=true`).
- Label a container `com.centurylinklabs.watchtower.schedule` with a schedule.
- That container is checked for updates on its own schedule, and the global
  schedule checks the rest.
- Optionally, turn on `--watch-events` (or `WATCHTOWER_WATCH_EVENTS=true`) so
  changes are picked up the moment they happen.

## Usage

### Docker Compose

```yaml
services:
  watchtower:
    image: ghcr.io/andrewrduncan/watchtower:latest
    restart: always
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - WATCHTOWER_CONTAINER_SCHEDULES=true
      - WATCHTOWER_WATCH_EVENTS=true
      - WATCHTOWER_SCHEDULE=0 0 4 * * *      # everything else: daily at 4am
      - WATCHTOWER_CLEANUP=true

  my-app:
    image: ghcr.io/example/my-app:latest
    labels:
      - com.centurylinklabs.watchtower.schedule=@every 10m

  my-api:
    image: ghcr.io/example/my-api:latest
    labels:
      - com.centurylinklabs.watchtower.schedule=@every 10m   # checked together with my-app

  database:
    image: postgres:17
    # no label: follows the daily schedule
```

### docker run

```console
$ docker run --detach --name watchtower \
    --volume /var/run/docker.sock:/var/run/docker.sock \
    ghcr.io/andrewrduncan/watchtower:latest \
    --container-schedules --watch-events --schedule "0 0 4 * * *"

$ docker run --detach --name my-app \
    --label "com.centurylinklabs.watchtower.schedule=@every 10m" \
    ghcr.io/example/my-app:latest
```

### The options

| Argument | Environment variable | Default | What it does |
|---|---|---|---|
| `--container-schedules` | `WATCHTOWER_CONTAINER_SCHEDULES` | `false` | Containers follow their schedule label |
| `--watch-events` | `WATCHTOWER_WATCH_EVENTS` | `false` | Follow Docker's container events |

Each works without the other. With both off, this fork behaves exactly like
upstream.

### The label

`com.centurylinklabs.watchtower.schedule` takes the same syntax as
`--schedule`:

| Value | Meaning |
|---|---|
| `@every 10m` | every 10 minutes (any Go duration: `30s`, `2h`, `1h30m`) |
| `@hourly`, `@daily`, `@weekly` | on the hour; at midnight; at midnight on Sundays |
| `*/15 * * * *` | cron, five fields: every 15 minutes |
| `0 30 9 * * 1-5` | cron with seconds, six fields: 9:30am on weekdays |

Quotes and surrounding spaces are ignored, so `"@every 10m"` from a compose file
works.

## How it behaves

### Container schedules

- **Every run is an ordinary Watchtower run.** It covers all the containers
  Watchtower manages, as upstream's scheduled run does, but checks for new
  images only the containers whose schedule fired. Schedules that fire at the
  same moment, the global one included, share one run.
- **Dependencies work exactly as upstream.** When a container is updated, the
  containers that depend on it (links, `network_mode: container:`,
  `depends-on` labels, Compose `depends_on`) restart with it, in dependency
  order, whatever their own schedule. They are not checked for new images
  until their own schedule fires.
- **Labels are read from the container list each run already makes**, at
  startup and after every run. Without `--watch-events`, a new label takes
  effect after the next run of any schedule. Nothing polls in between.
- **The rest of your configuration still applies.** A labeled container
  outside this Watchtower's [scope](https://watchtower.nickfedor.com/configuration/container-selection/#scope_filter),
  or not enabled while
  [label-enable](https://watchtower.nickfedor.com/configuration/container-selection/#enable_label_filter)
  is on, is not updated. So do monitor-only, no-pull, lifecycle hooks and
  notifications.
- **One update at a time.** Runs never overlap: a run that comes due while
  another is going waits for it, as upstream's scheduled runs do.
- **No global schedule needed.** With only `--container-schedules` and no
  `--schedule`, labeled containers update on their own schedules and the rest
  are not updated on a schedule.
- **Invalid schedules are skipped safely.** A label that doesn't parse is
  logged as a warning and the container stays on the global schedule.
- **The HTTP API is unchanged.** An update request covers every container it
  selects, whatever its schedule.

### Watch events

- **New containers are checked straight away.** A container created or
  recreated (by `docker compose up`, say) is checked for updates once no other
  container has been created for 10 seconds, at most a minute after the
  first, so a whole Compose project is checked in one run. Dependencies work
  as above.
- **Watchtower's own work is ignored.** Watchtower recreates the containers
  it updates, which Docker reports as new. A container created while an update
  runs, or just after, isn't checked again.
- **With container schedules,** the labels are read again two seconds after
  any container is created, removed or renamed, so a new schedule starts at
  once.
- **It recovers on its own.** When the event stream drops (Docker restarting,
  say), Watchtower reconnects, waiting 1, 2, 4… seconds up to a minute, and
  picks up from the last event it saw. A single drop is logged at debug level;
  it warns only when reconnecting keeps failing.
- **Requirements:** access to Docker's event stream, which a mounted Docker
  socket gives. Behind a socket proxy, allow events, for example `EVENTS=1` for
  [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy).

### What you'll see in the logs

```text
level=info msg="Containers follow their own schedule" schedule="@every 10m"
level=info msg="New containers are checked for updates as they are created"
level=info msg="Checking new containers for updates" containers=["my-app"]
level=debug msg="No container follows this schedule any more" schedule="@every 10m"
```

## Images

`ghcr.io/andrewrduncan/watchtower`, for `linux/amd64` and `linux/arm64`, public.

| Tag | What it is |
|---|---|
| `latest` | the newest build |
| `v1.24.0-schedules.0abe15f` | upstream release v1.24.0 with the features at commit `0abe15f` |
| `v1.24.0-main.0eca64f-schedules.0abe15f` | upstream's `main` at `0eca64f` (newer than v1.24.0) with the features, when they need upstream changes not yet released |

Pin a specific tag if you want to choose when Watchtower itself changes.

## How this fork is kept up to date

- `container-schedules` and `container-events` are the two features, each on
  upstream's `main` and independent of the other. They are what's offered
  upstream.
- `fork-features` is both, plus the small change that lets events refresh the
  schedules. It is what the image is built from.
- `fork` (the default branch) holds this README and the build.
- `main` mirrors upstream's `main`, kept identical every hour by
  [`sync-main.yml`](.github/workflows/sync-main.yml).

[`fork-build.yml`](.github/workflows/fork-build.yml) runs every day, and by
hand. It takes upstream's latest release (or its `main`, when the features need
newer changes), rebases `fork-features` onto it, and runs upstream's own checks
on the result before publishing the image:

- its full test suite;
- golangci-lint with its configuration;
- govulncheck, for known vulnerabilities the code calls;
- gosec, the security scanner. Upstream runs it in report-only mode and has
  findings of its own, so only findings on lines this fork adds stop the
  build; the rest are listed in the run's summary.

A build that already exists is skipped. Everything happens on GitHub.

When the features no longer apply cleanly to upstream, or a test fails, the
build opens an issue labeled `sync-conflict`. The last good image stays
published until it's fixed, and the next good build closes the issue.

## License and changes

Watchtower is licensed under the [Apache License 2.0](https://github.com/nicholas-fedor/watchtower/blob/main/LICENSE.md),
and so is this fork. Changes from upstream, all on the
[`fork-features`](https://github.com/andrewrduncan/watchtower/tree/fork-features)
branch:

- **New:**
  - `internal/update/schedule/containers.go`: container schedules;
  - `internal/update/schedule/created.go`: checking new containers;
  - `internal/container/watch`: following Docker's container events;
  - each with its tests.
- **Changed:**
  - the update session (`internal/update/update.go`), which can check only the
    containers that are due while keeping the rest in its dependency graph;
  - the scheduler (`internal/update/schedule/scheduling.go`) and its update
    lock, which records when it was last held;
  - its startup (`cmd/root.go`);
  - the Docker client, which gained an `Events` method;
  - the flag, setting and label definitions;
  - the documentation of scheduling and container labels;
  - the stored flag and help listings the tests compare against.
