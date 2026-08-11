package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"goanalytics/services/worker/internal/domain/subject"
)

// TestSubjectBootstrapVolume100k certifica el volumen MVP en PostgreSQL real.
func TestSubjectBootstrapVolume100k(t *testing.T) {
	if os.Getenv("SUBJECT_VOLUME_TEST") != "1" {
		t.Skip("SUBJECT_VOLUME_TEST no habilitado")
	}
	_, pool := newIntegrationEventRepository(t)
	repository, err := NewSubjectBootstrapRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	state, err := repository.BeginOrResume(ctx)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	const total = 100_000
	const pageSize = 500
	for offset := 0; offset < total; offset += pageSize {
		rows := make([]subject.SnapshotRow, 0, pageSize)
		for index := offset; index < offset+pageSize; index++ {
			status := subject.LifecycleActive
			if index%10 == 0 {
				status = subject.LifecycleInactive
			}
			rows = append(rows, subject.SnapshotRow{
				TenantID:          fmt.Sprintf("tenant-%02d", index%20),
				ExternalSubjectID: fmt.Sprintf("customer-%06d", index),
				LifecycleStatus:   status,
				SourceUpdatedAt:   time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC),
				SourceMutationID:  fmt.Sprintf("snapshot-%06d", index),
			})
		}
		page := subject.SnapshotPage{SourceRevision: "volume-revision-1", NextCursor: fmt.Sprintf("offset-%d", offset+pageSize), HasMore: offset+pageSize < total, Rows: rows}
		state, err = repository.ApplyPage(ctx, state, page)
		if err != nil {
			t.Fatalf("página %d: %v", offset/pageSize, err)
		}
		if offset+pageSize == total/2 {
			repository, err = NewSubjectBootstrapRepository(pool)
			if err != nil {
				t.Fatal(err)
			}
			state, err = repository.BeginOrResume(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if state.ProcessedCount != total/2 || state.Cursor != fmt.Sprintf("offset-%d", total/2) {
				t.Fatalf("checkpoint=%#v", state)
			}
		}
	}
	duration := time.Since(started)
	var count, active, inactive, distinct int
	if err = pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE lifecycle_status='active'),count(*) FILTER(WHERE lifecycle_status='inactive'),count(DISTINCT (tenant_id,external_subject_id)) FROM analytics_subjects`).Scan(&count, &active, &inactive, &distinct); err != nil {
		t.Fatal(err)
	}
	if count != total || distinct != total || active != 90_000 || inactive != 10_000 {
		t.Fatalf("count=%d distinct=%d active=%d inactive=%d", count, distinct, active, inactive)
	}
	if state.ProcessedCount != total {
		t.Fatalf("processed=%d", state.ProcessedCount)
	}
	if duration > 60*time.Minute {
		t.Fatalf("duración=%s supera 60m", duration)
	}
	t.Logf("backfill_subjects=%d duration=%s throughput=%.2f/s", total, duration, float64(total)/duration.Seconds())
}
