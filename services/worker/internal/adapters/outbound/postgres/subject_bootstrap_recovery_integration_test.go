package postgres

import (
	"context"
	"errors"
	"testing"

	"goanalytics/services/worker/internal/domain/subject"
)

func TestBootstrapIntegrationResumesFailureAndRequiresExplicitNewSnapshot(t *testing.T) {
	_, pool := newIntegrationEventRepository(t)
	repository, _ := NewSubjectBootstrapRepository(pool)
	ctx := context.Background()
	lock, acquired, err := repository.TryBootstrapLock(ctx)
	if err != nil || !acquired {
		t.Fatalf("lock=%v err=%v", acquired, err)
	}
	defer lock.Release(ctx)
	other, acquired, err := repository.TryBootstrapLock(ctx)
	if err != nil || acquired {
		if other != nil {
			_ = other.Release(ctx)
		}
		t.Fatalf("segundo lock=%v err=%v", acquired, err)
	}
	state, err := repository.BeginOrResume(ctx)
	if err != nil {
		t.Fatal(err)
	}
	state, err = repository.ApplyPage(ctx, state, subject.SnapshotPage{SourceRevision: "r1", NextCursor: "page2", HasMore: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.Fail(ctx, state, "red temporal"); err != nil {
		t.Fatal(err)
	}
	resumed, err := repository.BeginOrResume(ctx)
	if err != nil || resumed.ID != state.ID || resumed.Cursor != "page2" {
		t.Fatalf("resumed=%#v err=%v", resumed, err)
	}
	if err = repository.Fail(ctx, state, "snapshot_revision_expired"); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.BeginOrResume(ctx); !errors.Is(err, subject.ErrBootstrapExpired) {
		t.Fatalf("err=%v", err)
	}
	fresh, err := repository.BeginOrResume(ctx, true)
	if err != nil || fresh.ID == state.ID || fresh.Cursor != "" {
		t.Fatalf("fresh=%#v err=%v", fresh, err)
	}
	if _, err = repository.ApplyPage(ctx, fresh, subject.SnapshotPage{SourceRevision: "r2"}); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.BeginOrResume(ctx); !errors.Is(err, subject.ErrBootstrapCompleted) {
		t.Fatalf("err=%v", err)
	}
}
