package usecases

import (
	"context"
	"errors"
	"testing"

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
}

func (fake *bootstrapRepoFake) BeginOrResume(context.Context) (subject.BootstrapState, error) {
	return fake.state, nil
}
func (fake *bootstrapRepoFake) ApplyPage(_ context.Context, state subject.BootstrapState, page subject.SnapshotPage) (subject.BootstrapState, error) {
	fake.applied++
	state.Cursor = page.NextCursor
	state.SourceRevision = page.SourceRevision
	return state, nil
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

func TestBootstrapSubjectsRestartsExpiredRevision(t *testing.T) {
	source := &snapshotSourceFake{errors: []error{ErrSnapshotRevisionExpired, nil}, pages: []subject.SnapshotPage{{}, {SourceRevision: "r2", HasMore: false}}}
	repo := &bootstrapRepoFake{state: subject.BootstrapState{ID: 1, Cursor: "old", SourceRevision: "r1"}}
	useCase, _ := NewBootstrapSubjects(source, repo, nil, 100)
	if err := useCase.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.restarted != 1 || repo.applied != 1 {
		t.Fatalf("restarted=%d applied=%d", repo.restarted, repo.applied)
	}
}

func TestBootstrapSubjectsFailsAfterRepeatedExpiration(t *testing.T) {
	source := &snapshotSourceFake{errors: []error{ErrSnapshotRevisionExpired, ErrSnapshotRevisionExpired, ErrSnapshotRevisionExpired}}
	repo := &bootstrapRepoFake{state: subject.BootstrapState{ID: 1}}
	useCase, _ := NewBootstrapSubjects(source, repo, nil, 100)
	err := useCase.Run(context.Background())
	if !errors.Is(err, ErrSnapshotRevisionExpired) || repo.restarted != 2 || repo.failed == "" {
		t.Fatalf("err=%v restarted=%d failed=%q", err, repo.restarted, repo.failed)
	}
}
