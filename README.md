# Watchtower, with per-container schedules

This is a fork of [Watchtower](https://github.com/nicholas-fedor/watchtower),
which automatically updates running Docker containers when their images change.
It adds one feature: **a container can set its own update schedule**, so you
can check some containers every few minutes and leave the rest on a daily
schedule, all with one Watchtower.

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
is for this fork not to need to exist: the feature is offered upstream, for
[issue #1589](https://github.com/nicholas-fedor/watchtower/issues/1589).

## What it adds

Upstream Watchtower checks every container on one schedule (`--schedule` or
`--interval`). Some containers deserve more attention than others: an app you
deploy several times a day versus a database you'd rather leave alone. Until
now that meant running a second Watchtower with its own scope.

With this fork:

- Turn on `--container-schedules` (or `WATCHTOWER_CONTAINER_SCHEDULES=true`).
- Label a container `com.centurylinklabs.watchtower.schedule` with a schedule.
- That container is updated on its own schedule and left out of the global
  one. Every other container carries on as before.

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
    --container-schedules --schedule "0 0 4 * * *"

$ docker run --detach --name my-app \
    --label "com.centurylinklabs.watchtower.schedule=@every 10m" \
    ghcr.io/example/my-app:latest
```

### The option

| | |
|---|---|
| Argument | `--container-schedules` |
| Environment variable | `WATCHTOWER_CONTAINER_SCHEDULES` |
| Type | Boolean |
| Default | `false` |

It is off by default. With it off, this fork behaves exactly like upstream: no
label is read and no extra connection to Docker is opened.

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

- **One run per schedule.** Containers that share a schedule are checked
  together in one run. Each run covers only its own containers, and the global
  schedule's runs leave all labeled containers out.
- **The rest of your configuration still applies.** A labeled container
  outside this Watchtower's [scope](https://watchtower.nickfedor.com/configuration/container-selection/#scope_filter),
  or not enabled while
  [label-enable](https://watchtower.nickfedor.com/configuration/container-selection/#enable_label_filter)
  is on, is not updated. So do monitor-only, no-pull, lifecycle hooks and
  notifications.
- **One update at a time.** Runs never overlap: a run that comes due while
  another is going waits for it, as upstream's scheduled runs do.
- **No global schedule needed.** With only `--container-schedules`, labeled
  containers update on their own schedules and the rest are not updated on a
  schedule.
- **Invalid schedules are skipped safely.** A label that doesn't parse is
  logged as a warning and the container stays on the global schedule.
- **The HTTP API is unchanged.** An update request covers every container it
  selects, whatever its schedule.

### Changes take effect at once, with nothing polled

Watchtower follows Docker's container events. It lists the containers once at
startup, then hears about each container as it is created, removed or renamed.
Add a label (by recreating the container, as any label change requires) and its
schedule starts straight away. Remove the last container on a schedule and the
schedule goes. Between events and due runs, nothing runs.

### It recovers on its own

A connection held open for days has to survive things going wrong:

- **Docker restarts, or the connection drops:** Watchtower reconnects, waiting
  1, 2, 4… seconds up to a minute, then lists the containers again, since
  events missed meanwhile are not replayed.
- **A connection that dies without an error** (possible with a remote Docker
  over a network): after every update run the containers are listed again, so
  anything missed is caught at the next run. This adds no polling.
- **Quiet logs:** a single dropped connection that reconnects straight away is
  routine (a Docker restart, a proxy's idle timeout) and logged at debug level.
  Watchtower warns only when reconnecting keeps failing, and logs when it has
  recovered.
- **Faults stay contained:** if the event listener ever fails or panics, it is
  logged and restarted. The rest of Watchtower keeps working throughout, and
  labeled containers simply wait until their schedules are back.

### Requirements

- Access to Docker's event stream, which a mounted Docker socket gives. Behind
  a socket proxy, allow events, for example `EVENTS=1` for
  [docker-socket-proxy](https://github.com/Tecnativa/docker-socket-proxy).
- Docker Engine with API v1.43 or later, as upstream.

### What you'll see in the logs

```text
level=info msg="Containers with a schedule label follow their own schedule"
level=info msg="Scheduled updates for containers with their own schedule" schedule="@every 10m"
level=debug msg="Container follows its own schedule" container=my-app schedule="@every 10m"
level=info msg="No containers left on a container schedule: removed it" schedule="@every 10m"
```

## Images

`ghcr.io/andrewrduncan/watchtower`, for `linux/amd64` and `linux/arm64`, public.

| Tag | What it is |
|---|---|
| `latest` | the newest build |
| `v1.24.0-schedules.0abe15f` | upstream release v1.24.0 with the feature at commit `0abe15f` |
| `v1.24.0-main.0eca64f-schedules.0abe15f` | upstream's `main` at `0eca64f` (newer than v1.24.0) with the feature, when the feature needs upstream changes not yet released |

Pin a specific tag if you want to choose when Watchtower itself changes.

## How this fork is kept up to date

- `per-container-schedule` is the feature, on upstream's `main`. It is what's
  offered upstream as a pull request.
- `fork` (the default branch) holds this README and the build.
- `main` mirrors upstream's `main`, kept identical every hour by
  [`sync-main.yml`](.github/workflows/sync-main.yml).

[`fork-build.yml`](.github/workflows/fork-build.yml) runs every day, and by
hand. It takes upstream's latest release (or its `main`, when the feature needs
newer changes), rebases the feature onto it, and runs upstream's own checks on
the result before publishing the image:

- its full test suite;
- golangci-lint with its configuration;
- govulncheck, for known vulnerabilities the code calls;
- gosec, the security scanner. Upstream runs it in report-only mode and has
  findings of its own, so only findings on lines this fork adds stop the
  build; the rest are listed in the run's summary. A build that already exists is skipped. Everything
happens on GitHub.

When the feature no longer applies cleanly to upstream, or a test fails, the
build opens an issue labeled `sync-conflict`. The last good image stays
published until it's fixed, and the next good build closes the issue.

## License and changes

Watchtower is licensed under the [Apache License 2.0](https://github.com/nicholas-fedor/watchtower/blob/main/LICENSE.md),
and so is this fork. Changes from upstream, all on the
[`per-container-schedule`](https://github.com/andrewrduncan/watchtower/tree/per-container-schedule)
branch:

- **New:** `internal/update/schedule/percontainer` (the label rules, the
  registry of schedules, and the Docker event source, with their tests).
- **Changed:**
  - the scheduler (`internal/update/schedule/scheduling.go`), to run container
    schedules and supervise the event source;
  - its startup (`cmd/root.go`);
  - the Docker client, which gained an `Events` method;
  - the flag, setting and label definitions;
  - the documentation of scheduling and container labels;
  - the stored flag and help listings the tests compare against.
