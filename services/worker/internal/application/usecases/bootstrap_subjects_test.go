package usecases

import (
	"context"
	"errors"
	"testing"

	"goanalytics/services/worker/internal/application/ports/outbound"
	"goanalytics/services/worker/internal/domain/subject"
)

type snapshotSourceFake struct {
	pages  []subject.SnapshotPage
	errors []error
	calls  int
}

func (fake *snapshotSourceFake) FetchPage(context.Context, string, string, int) (subject.SnapshotPage, error) {
	index := fake.calls
	fake.calls++
	if index < len(fake.errors) && fake.errors[index] != nil {
		return subject.SnapshotPage{}, fake.errors[index]
	}
	return fake.pages[index], nil
}

type bootstrapRepoFake struct {
	state     subject.BootstrapState
	applied   int
	restarted int
	failed    string
	applyErr  error
}

func (fake *bootstrapRepoFake) BeginOrResume(_ context.Context, force ...bool) (subject.BootstrapState, error) {
	if len(force) > 0 && force[0] {
		fake.restarted++
	}
	return fake.state, nil
}

type bootstrapLockFake struct{}

func (bootstrapLockFake) Release(context.Context) error { return nil }
func (fake *bootstrapRepoFake) TryBootstrapLock(context.Context) (outbound.ProjectionLock, bool, error) {
	return bootstrapLockFake{}, true, nil
}
func (fake *bootstrapRepoFake) ApplyPage(_ context.Context, state subject.BootstrapState, page subject.SnapshotPage) (subject.BootstrapState, error) {
	fake.applied++
	if fake.applyErr != nil {
		return state, fake.applyErr
	}
	state.Cursor = page.NextCursor
	state.SourceRevision = page.SourceRevision
	return state, nil
}

func TestBootstrapSubjectsPersistsApplyFailure(t *testing.T) {
	source := &snapshotSourceFake{pages: []subject.SnapshotPage{{SourceRevision: "r1"}}}
	repo := &bootstrapRepoFake{state: subject.BootstrapState{ID: 1}, applyErr: errors.New("postgres unavailable")}
	useCase, _ := NewBootstrapSubjects(source, repo, nil, 100)
	if err := useCase.Run(context.Background()); err == nil || repo.failed == "" {
		t.Fatalf("err=%v failed=%q", err, repo.failed)
	}
}
func (fake *bootstrapRepoFake) RestartExpired(context.Context, subject.BootstrapState) (subject.BootstrapState, error) {
	fake.restarted++
	return subject.BootstrapState{ID: int64(fake.restarted + 1)}, nil
}
func (fake *bootstrapRepoFake) Fail(_ context.Context, _ subject.BootstrapState, cause string) error {
	fake.failed = cause
	return nil
}

func TestBootstrapSubjectsResumesUntilLastPage(t *testing.T) {
	source := &snapshotSourceFake{pages: []subject.SnapshotPage{{SourceRevision: "r1", NextCursor: "c2", HasMore: true}, {SourceRevision: "r1", HasMore: false}}}
	repo := &bootstrapRepoFake{state: subject.BootstrapState{ID: 1, Cursor: "c1", SourceRevision: "r1"}}
	useCase, err := NewBootstrapSubjects(source, repo, nil, 500)
	if err != nil {
		t.Fatal(err)
	}
	if err = useCase.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.calls != 2 || repo.applied != 2 {
		t.Fatalf("calls=%d applied=%d", source.calls, repo.applied)
	}
}

func TestBootstrapSubjectsRequiresExplicitRestartAfterExpiration(t *testing.T) {
	source := &snapshotSourceFake{errors: []error{ErrSnapshotRevisionExpired, nil}, pages: []subject.SnapshotPage{{}, {SourceRevision: "r2", HasMore: false}}}
	repo := &bootstrapRepoFake{state: subject.BootstrapState{ID: 1, Cursor: "old", SourceRevision: "r1"}}
	useCase, _ := NewBootstrapSubjects(source, repo, nil, 100)
	if err := useCase.Run(context.Background()); !errors.Is(err, ErrSnapshotRevisionExpired) {
		t.Fatal(err)
	}
	if repo.restarted != 0 || repo.applied != 0 || repo.failed != "snapshot_revision_expired" {
		t.Fatalf("restarted=%d applied=%d", repo.restarted, repo.applied)
	}
}

func TestBootstrapSubjectsSupportsExplicitNewSnapshot(t *testing.T) {
	source := &snapshotSourceFake{pages: []subject.SnapshotPage{{SourceRevision: "new"}}}
	repo := &bootstrapRepoFake{state: subject.BootstrapState{ID: 1}}
	useCase, _ := NewBootstrapSubjects(source, repo, nil, 100)
	err := useCase.Run(context.Background(), true)
	if err != nil || repo.restarted != 1 || repo.applied != 1 {
		t.Fatalf("err=%v restarted=%d failed=%q", err, repo.restarted, repo.failed)
	}
}
