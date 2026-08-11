package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/subject"
)

type projectionLockFake struct{ released bool }

func (lock *projectionLockFake) Release(context.Context) error { lock.released = true; return nil }

type projectionRepositoryFake struct {
	lock         *projectionLockFake
	acquired     bool
	checkpoint   subject.ProjectionWatermark
	batches      [][]subject.AffectedDay
	watermarks   []subject.ProjectionWatermark
	reads        []int64
	index        int
	recomputeErr error
	completed    bool
	failed       bool
	skipped      bool
}

func (fake *projectionRepositoryFake) TryLock(context.Context, string, int) (outbound.ProjectionLock, bool, error) {
	return fake.lock, fake.acquired, nil
}
func (fake *projectionRepositoryFake) RecordSkippedLocked(context.Context, string, int) error {
	fake.skipped = true
	return nil
}
func (fake *projectionRepositoryFake) StartRun(context.Context, string, int, subject.ProjectionWatermark) (subject.ProjectionRun, error) {
	return subject.ProjectionRun{ID: 1}, nil
}
func (fake *projectionRepositoryFake) CompleteRun(context.Context, int64, subject.ProjectionWatermark, int, int64, int64) error {
	fake.completed = true
	return nil
}
func (fake *projectionRepositoryFake) FailRun(context.Context, int64, string) error {
	fake.failed = true
	return nil
}
func (fake *projectionRepositoryFake) LoadCheckpoint(context.Context, string, int) (subject.ProjectionWatermark, error) {
	return fake.checkpoint, nil
}
func (fake *projectionRepositoryFake) LoadAffectedDays(context.Context, subject.ProjectionWatermark, int) ([]subject.AffectedDay, subject.ProjectionWatermark, int64, error) {
	index := fake.index
	fake.index++
	if index >= len(fake.batches) {
		return nil, fake.checkpoint, 0, nil
	}
	return fake.batches[index], fake.watermarks[index], fake.reads[index], nil
}
func (fake *projectionRepositoryFake) RecomputeDaysAndCheckpoint(context.Context, string, int, []subject.AffectedDay, subject.ProjectionWatermark) (int64, error) {
	if fake.recomputeErr != nil {
		return 0, fake.recomputeErr
	}
	return 1, nil
}

func TestProjectGlobalProcessesIncrementalBatches(t *testing.T) {
	day := subject.AffectedDay{TenantID: "tenant-1", SubjectID: 1, LocalDate: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), TimezoneName: "UTC"}
	repository := &projectionRepositoryFake{lock: &projectionLockFake{}, acquired: true, batches: [][]subject.AffectedDay{{day}, {day}}, watermarks: []subject.ProjectionWatermark{{UpdatedAt: time.Now(), ID: 2}, {UpdatedAt: time.Now(), ID: 3}}, reads: []int64{2, 1}}
	projector, _ := NewProjectGlobal(repository, nil, 1, 2)
	if err := projector.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !repository.completed || repository.failed || !repository.lock.released || repository.index != 2 {
		t.Fatalf("repo=%#v", repository)
	}
}

func TestProjectGlobalDoesNotAdvanceAfterRecomputeFailure(t *testing.T) {
	repository := &projectionRepositoryFake{lock: &projectionLockFake{}, acquired: true, batches: [][]subject.AffectedDay{{{TenantID: "tenant-1"}}}, watermarks: []subject.ProjectionWatermark{{ID: 2}}, reads: []int64{1}, recomputeErr: errors.New("postgres")}
	projector, _ := NewProjectGlobal(repository, nil, 1, 10)
	if err := projector.Run(context.Background()); err == nil || !repository.failed || repository.completed {
		t.Fatalf("err=%v repo=%#v", err, repository)
	}
}

func TestProjectGlobalRecordsSkippedLock(t *testing.T) {
	repository := &projectionRepositoryFake{acquired: false}
	projector, _ := NewProjectGlobal(repository, nil, 1, 10)
	if err := projector.Run(context.Background()); err != nil || !repository.skipped {
		t.Fatalf("err=%v skipped=%v", err, repository.skipped)
	}
}
