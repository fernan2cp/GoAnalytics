package usecases

import (
	"context"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

type schedulerRepositoryFake struct {
	projectionRepositoryFake
	completedAt time.Time
}

func (fake *schedulerRepositoryFake) LastCompletedAt(context.Context, string, int) (time.Time, error) {
	return fake.completedAt, nil
}

type schedulerRunnerFake struct {
	calls int
	err   error
}

func (fake *schedulerRunnerFake) Run(context.Context) error { fake.calls++; return fake.err }

func TestProjectionSchedulerRunsAtStartupWithoutSuccess(t *testing.T) {
	repository := &schedulerRepositoryFake{}
	runner := &schedulerRunnerFake{}
	scheduler := NewProjectionScheduler(repository, runner, 1, 24*time.Hour, time.Minute)
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	ran, err := scheduler.RunIfDue(context.Background(), now)
	if err != nil || !ran || runner.calls != 1 || !scheduler.Status().LastSuccessAt.Equal(now) {
		t.Fatalf("ran=%v err=%v calls=%d status=%#v", ran, err, runner.calls, scheduler.Status())
	}
}

func TestProjectionSchedulerSkipsFreshRun(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	repository := &schedulerRepositoryFake{completedAt: now.Add(-time.Hour)}
	runner := &schedulerRunnerFake{}
	scheduler := NewProjectionScheduler(repository, runner, 1, 24*time.Hour, time.Minute)
	ran, err := scheduler.RunIfDue(context.Background(), now)
	if err != nil || ran || runner.calls != 0 {
		t.Fatalf("ran=%v err=%v calls=%d", ran, err, runner.calls)
	}
}

var _ outbound.GlobalProjectionRepository = (*schedulerRepositoryFake)(nil)
