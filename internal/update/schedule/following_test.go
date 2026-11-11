package schedule

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/moby/moby/api/types/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicholas-fedor/watchtower/internal/observability/logging"
)

// TestContainerSchedules_FollowContainerEvents verifies that with container
// events followed, a new container's schedule takes effect moments after it
// starts, without waiting for a session.
func TestContainerSchedules_FollowContainerEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		list := &containerList{}
		list.set(unlabeled("a"))

		changes := newFakeChanges()
		recorder := &runRecorder{list: list}

		deps := testDeps(t, recorder.runUpdate, func(context.Context, logging.StartupParams) {})
		deps.ScheduleSpec = "@every 1h"
		deps.ContainerSchedules = list.list
		deps.ContainerChanges = changes.follow

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)

		go func() { done <- RunUpgradesOnSchedule(ctx, deps) }()

		time.Sleep(5 * time.Second)
		synctest.Wait()

		list.set(unlabeled("a"), labeled("b", "@every 1s"))
		changes.send(events.ActionStart, "b")

		time.Sleep(4 * time.Second)
		synctest.Wait()
		cancel()
		require.NoError(t, <-done)

		runs := recorder.snapshot()
		require.NotEmpty(t, runs, "b's schedule did not run before the global schedule")

		for _, run := range runs {
			assert.Equal(t, []string{"b"}, run.due)
		}
	})
}
