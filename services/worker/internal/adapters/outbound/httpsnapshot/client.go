package httpsnapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"goanalytics/services/worker/internal/application/usecases"
	"goanalytics/services/worker/internal/domain/subject"
)

// Client consume el snapshot interno autenticado del proyecto principal.
type Client struct {
	endpoint string
	token    string
	http     *http.Client
}

// NewClient crea un cliente con timeout acotado.
func NewClient(endpoint, token string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(endpoint) == "" || strings.TrimSpace(token) == "" || timeout <= 0 {
		return nil, fmt.Errorf("configuración de snapshot incompleta")
	}
	return &Client{endpoint: endpoint, token: token, http: &http.Client{Timeout: timeout}}, nil
}

// FetchPage obtiene y valida una página del contrato revisionado.
func (client *Client) FetchPage(ctx context.Context, cursor, revision string, limit int) (subject.SnapshotPage, error) {
	parsed, err := url.Parse(client.endpoint)
	if err != nil {
		return subject.SnapshotPage{}, err
	}
	query := parsed.Query()
	query.Set("subject_type", "customer")
	query.Set("limit", strconv.Itoa(limit))
	if cursor != "" || revision != "" {
		query.Set("cursor", cursor)
		query.Set("source_revision", revision)
	}
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return subject.SnapshotPage{}, err
	}
	request.Header.Set("Authorization", "Bearer "+client.token)
	response, err := client.doWithRetries(request)
	if err != nil {
		return subject.SnapshotPage{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return subject.SnapshotPage{}, usecases.ErrSnapshotRevisionExpired
	}
	if response.StatusCode != http.StatusOK {
		return subject.SnapshotPage{}, fmt.Errorf("snapshot respondió estado %d", response.StatusCode)
	}
	var payload snapshotResponse
	decoder := json.NewDecoder(response.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return subject.SnapshotPage{}, err
	}
	return payload.toDomain()
}

func (client *Client) doWithRetries(request *http.Request) (*http.Response, error) {
	backoff := []time.Duration{0, 100 * time.Millisecond, 500 * time.Millisecond}
	var lastErr error
	for attempt, delay := range backoff {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-request.Context().Done():
				timer.Stop()
				return nil, request.Context().Err()
			case <-timer.C:
			}
		}
		response, err := client.http.Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		if (response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500) && attempt < len(backoff)-1 {
			response.Body.Close()
			continue
		}
		return response, nil
	}
	return nil, lastErr
}

type snapshotResponse struct {
	SubjectType    string        `json:"subject_type"`
	SourceRevision string        `json:"source_revision"`
	NextCursor     *string       `json:"next_cursor"`
	HasMore        bool          `json:"has_more"`
	Subjects       []snapshotRow `json:"subjects"`
}

type snapshotRow struct {
	TenantID                    string  `json:"tenant_id"`
	ExternalSubjectID           string  `json:"external_subject_id"`
	LifecycleStatus             string  `json:"lifecycle_status"`
	SourceUpdatedAt             string  `json:"source_updated_at"`
	SourceMutationID            string  `json:"source_mutation_id"`
	MergedIntoExternalSubjectID *string `json:"merged_into_external_subject_id"`
}

func (payload snapshotResponse) toDomain() (subject.SnapshotPage, error) {
	if payload.SubjectType != "customer" || payload.SourceRevision == "" || (payload.HasMore && payload.NextCursor == nil) {
		return subject.SnapshotPage{}, fmt.Errorf("contrato de snapshot inválido")
	}
	page := subject.SnapshotPage{SourceRevision: payload.SourceRevision, HasMore: payload.HasMore}
	if payload.NextCursor != nil {
		page.NextCursor = *payload.NextCursor
	}
	for _, row := range payload.Subjects {
		updatedAt, err := time.Parse(time.RFC3339Nano, row.SourceUpdatedAt)
		if err != nil || row.TenantID == "" || row.ExternalSubjectID == "" || row.SourceMutationID == "" {
			return subject.SnapshotPage{}, fmt.Errorf("sujeto de snapshot inválido")
		}
		status := subject.LifecycleStatus(row.LifecycleStatus)
		merged := ""
		if row.MergedIntoExternalSubjectID != nil {
			merged = *row.MergedIntoExternalSubjectID
		}
		if status != subject.LifecycleActive && status != subject.LifecycleInactive && status != subject.LifecycleMerged {
			return subject.SnapshotPage{}, fmt.Errorf("lifecycle de snapshot inválido")
		}
		if status == subject.LifecycleMerged && merged == "" {
			return subject.SnapshotPage{}, fmt.Errorf("destino de merge requerido")
		}
		page.Rows = append(page.Rows, subject.SnapshotRow{TenantID: row.TenantID, ExternalSubjectID: row.ExternalSubjectID, LifecycleStatus: status, SourceUpdatedAt: updatedAt, SourceMutationID: row.SourceMutationID, MergedIntoExternalSubjectID: merged})
	}
	return page, nil
}
