package subject

import (
	"testing"
	"time"
)

func TestLifecycleOrderIsAfterUsesTimestampAndMutationIDOnly(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	current := LifecycleOrder{SourceUpdatedAt: base, SourceMutationID: "mutation-20"}
	tests := []struct {
		name     string
		incoming LifecycleOrder
		want     bool
	}{
		{name: "timestamp posterior", incoming: LifecycleOrder{SourceUpdatedAt: base.Add(time.Microsecond), SourceMutationID: "mutation-01"}, want: true},
		{name: "timestamp anterior", incoming: LifecycleOrder{SourceUpdatedAt: base.Add(-time.Microsecond), SourceMutationID: "mutation-99"}},
		{name: "mutation posterior", incoming: LifecycleOrder{SourceUpdatedAt: base, SourceMutationID: "mutation-21"}, want: true},
		{name: "mutation anterior", incoming: LifecycleOrder{SourceUpdatedAt: base, SourceMutationID: "mutation-19"}},
		{name: "duplicado", incoming: current},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.incoming.IsAfter(current); got != test.want {
				t.Fatalf("IsAfter() = %v, esperado %v", got, test.want)
			}
		})
	}
}
