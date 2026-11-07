# Watchtower with per-container schedules

A fork of [nicholas-fedor/watchtower](https://github.com/nicholas-fedor/watchtower)
adding one feature: containers can set their own update schedule
([upstream issue #1589](https://github.com/nicholas-fedor/watchtower/issues/1589)).

```yaml
services:
  watchtower:
    image: ghcr.io/andrewrduncan/watchtower:latest
    environment:
      - WATCHTOWER_CONTAINER_SCHEDULES=true
  game:
    image: example/game:latest
    labels:
      - com.centurylinklabs.watchtower.schedule=@every 10m
```

Everything else is upstream Watchtower, unchanged. See
[Container Schedules](https://github.com/andrewrduncan/watchtower/blob/per-container-schedule/docs/configuration/scheduling/index.md#container-schedules)
for the details.

## Branches

- `per-container-schedule`: the feature, on upstream's `main`. This is what is
  offered upstream as a pull request.
- `fork` (default): this README and the build workflow.
- `main`: upstream's `main`, untouched.

## Builds

[`fork-build.yml`](.github/workflows/fork-build.yml) runs daily, on pushes, and
by hand. It takes upstream's latest release, rebases the feature onto it,
runs upstream's tests, and pushes `ghcr.io/andrewrduncan/watchtower` tagged
`latest` and `<release>-schedules.<commit>`. A build that already exists is
skipped.

When the feature no longer applies to a new release, or its tests fail, the
build opens an issue labeled `sync-conflict` with the details. The last good
image stays in place until it is fixed, and the next good build closes the
issue.

If upstream merges the feature, this fork goes back to being unnecessary.
