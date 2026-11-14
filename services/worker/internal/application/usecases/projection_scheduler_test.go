package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
)

type schedulerRepositoryFake struct {
	projectionRepositoryFake
	completedAt time.Time
	byName      map[string]time.Time
}

func (fake *schedulerRepositoryFake) LastCompletedAt(_ context.Context, name string, _ int) (time.Time, error) {
	if fake.byName != nil {
		return fake.byName[name], nil
	}
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

func TestSchedulerRetriesContextWhenGlobalIsFresh(t *testing.T) {
	now := time.Now().UTC()
	repository := &schedulerRepositoryFake{byName: map[string]time.Time{"subject_global": now, "subject_context": now.Add(-49 * time.Hour)}}
	runner := &schedulerRunnerFake{}
	scheduler := NewProjectionScheduler(repository, runner, 1, 24*time.Hour, time.Minute, "subject_global", "subject_context")
	ran, err := scheduler.RunIfDue(context.Background(), now)
	if err != nil || !ran || runner.calls != 1 {
		t.Fatalf("ran=%v err=%v calls=%d", ran, err, runner.calls)
	}
}

type recoveryRunner struct {
	calls  int
	cancel context.CancelFunc
}

func (runner *recoveryRunner) Run(context.Context) error {
	runner.calls++
	if runner.calls == 1 {
		return errors.New("fallo temporal")
	}
	runner.cancel()
	return nil
}

func TestSchedulerSurvivesTemporaryProjectionFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	runner := &recoveryRunner{cancel: cancel}
	scheduler := NewProjectionScheduler(&schedulerRepositoryFake{}, runner, 1, time.Hour, time.Millisecond)
	_ = scheduler.Serve(ctx)
	if runner.calls != 2 || scheduler.Status().LastError != "" || scheduler.Status().LastSuccessAt.IsZero() {
		t.Fatalf("calls=%d status=%#v", runner.calls, scheduler.Status())
	}
}
