package sdk

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// StreamQuery represents a query for filtering streams
type StreamQuery struct {
	// Filter by user ID
	UserID string `json:"user_id,omitempty"`

	// Filter by state
	State StreamState `json:"state,omitempty"`

	// Filter by protocol
	Protocol StreamProtocol `json:"protocol,omitempty"`

	// Filter by date range
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`

	// Filter by start date range
	StartedAfter  *time.Time `json:"started_after,omitempty"`
	StartedBefore *time.Time `json:"started_before,omitempty"`

	// Filter by viewer count range
	MinViewers *int64 `json:"min_viewers,omitempty"`
	MaxViewers *int64 `json:"max_viewers,omitempty"`

	// Search in title and description
	Search string `json:"search,omitempty"`

	// Metadata filters (key=value pairs)
	Metadata map[string]string `json:"metadata,omitempty"`

	// Sorting
	SortBy    string `json:"sort_by,omitempty"`    // created_at, started_at, viewer_count, duration
	SortOrder string `json:"sort_order,omitempty"` // asc, desc

	// Pagination
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

// StreamQueryResult represents the result of a stream query
type StreamQueryResult struct {
	Streams    []*Stream `json:"streams"`
	TotalCount int       `json:"total_count"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

// DefaultStreamQuery returns a query with default values
func DefaultStreamQuery() *StreamQuery {
	return &StreamQuery{
		SortBy:    "created_at",
		SortOrder: "desc",
		Offset:    0,
		Limit:     50,
	}
}

// StreamQueryBuilder provides a fluent interface for building stream queries
type StreamQueryBuilder struct {
	query *StreamQuery
}

// NewStreamQueryBuilder creates a new query builder
func NewStreamQueryBuilder() *StreamQueryBuilder {
	return &StreamQueryBuilder{
		query: DefaultStreamQuery(),
	}
}

// WithUserID filters by user ID
func (qb *StreamQueryBuilder) WithUserID(userID string) *StreamQueryBuilder {
	qb.query.UserID = userID
	return qb
}

// WithState filters by stream state
func (qb *StreamQueryBuilder) WithState(state StreamState) *StreamQueryBuilder {
	qb.query.State = state
	return qb
}

// WithProtocol filters by streaming protocol
func (qb *StreamQueryBuilder) WithProtocol(protocol StreamProtocol) *StreamQueryBuilder {
	qb.query.Protocol = protocol
	return qb
}

// CreatedAfter filters streams created after the given time
func (qb *StreamQueryBuilder) CreatedAfter(t time.Time) *StreamQueryBuilder {
	qb.query.CreatedAfter = &t
	return qb
}

// CreatedBefore filters streams created before the given time
func (qb *StreamQueryBuilder) CreatedBefore(t time.Time) *StreamQueryBuilder {
	qb.query.CreatedBefore = &t
	return qb
}

// StartedAfter filters streams started after the given time
func (qb *StreamQueryBuilder) StartedAfter(t time.Time) *StreamQueryBuilder {
	qb.query.StartedAfter = &t
	return qb
}

// StartedBefore filters streams started before the given time
func (qb *StreamQueryBuilder) StartedBefore(t time.Time) *StreamQueryBuilder {
	qb.query.StartedBefore = &t
	return qb
}

// WithMinViewers filters streams with at least the given viewer count
func (qb *StreamQueryBuilder) WithMinViewers(count int64) *StreamQueryBuilder {
	qb.query.MinViewers = &count
	return qb
}

// WithMaxViewers filters streams with at most the given viewer count
func (qb *StreamQueryBuilder) WithMaxViewers(count int64) *StreamQueryBuilder {
	qb.query.MaxViewers = &count
	return qb
}

// WithSearch searches in title and description
func (qb *StreamQueryBuilder) WithSearch(term string) *StreamQueryBuilder {
	qb.query.Search = term
	return qb
}

// WithMetadata filters by metadata key-value pairs
func (qb *StreamQueryBuilder) WithMetadata(key, value string) *StreamQueryBuilder {
	if qb.query.Metadata == nil {
		qb.query.Metadata = make(map[string]string)
	}
	qb.query.Metadata[key] = value
	return qb
}

// SortBy sets the sort field
func (qb *StreamQueryBuilder) SortBy(field string) *StreamQueryBuilder {
	qb.query.SortBy = field
	return qb
}

// SortOrder sets the sort order (asc or desc)
func (qb *StreamQueryBuilder) SortOrder(order string) *StreamQueryBuilder {
	qb.query.SortOrder = order
	return qb
}

// Offset sets the pagination offset
func (qb *StreamQueryBuilder) Offset(offset int) *StreamQueryBuilder {
	qb.query.Offset = offset
	return qb
}

// Limit sets the pagination limit
func (qb *StreamQueryBuilder) Limit(limit int) *StreamQueryBuilder {
	qb.query.Limit = limit
	return qb
}

// Build returns the built query
func (qb *StreamQueryBuilder) Build() *StreamQuery {
	return qb.query
}

// QueryStreams executes a stream query
func (sm *StreamManager) QueryStreams(ctx context.Context, query *StreamQuery) (*StreamQueryResult, error) {
	if query == nil {
		query = DefaultStreamQuery()
	}

	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Start with all streams
	streams := make([]*Stream, 0, len(sm.streams))
	for _, stream := range sm.streams {
		streams = append(streams, stream)
	}

	// Apply filters
	filtered := make([]*Stream, 0)
	for _, stream := range streams {
		if matchesQuery(stream, query) {
			filtered = append(filtered, stream)
		}
	}

	totalCount := len(filtered)

	// Sort streams
	sortStreams(filtered, query.SortBy, query.SortOrder)

	// Apply pagination
	start := query.Offset
	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + query.Limit
	if end > len(filtered) {
		end = len(filtered)
	}

	if query.Limit == 0 {
		end = len(filtered)
	}

	paginated := filtered[start:end]

	return &StreamQueryResult{
		Streams:    paginated,
		TotalCount: totalCount,
		Offset:     query.Offset,
		Limit:      query.Limit,
	}, nil
}

// matchesQuery checks if a stream matches the query criteria
func matchesQuery(stream *Stream, query *StreamQuery) bool {
	// Filter by user ID
	if query.UserID != "" && stream.UserID != query.UserID {
		return false
	}

	// Filter by state
	if query.State != "" && stream.State != query.State {
		return false
	}

	// Filter by protocol
	if query.Protocol != "" && stream.Protocol != query.Protocol {
		return false
	}

	// Filter by created date range
	if query.CreatedAfter != nil && stream.CreatedAt.Before(*query.CreatedAfter) {
		return false
	}

	if query.CreatedBefore != nil && stream.CreatedAt.After(*query.CreatedBefore) {
		return false
	}

	// Filter by started date range
	if query.StartedAfter != nil {
		if stream.StartedAt == nil || stream.StartedAt.Before(*query.StartedAfter) {
			return false
		}
	}

	if query.StartedBefore != nil {
		if stream.StartedAt == nil || stream.StartedAt.After(*query.StartedBefore) {
			return false
		}
	}

	// Filter by viewer count range
	if query.MinViewers != nil && stream.ViewerCount < *query.MinViewers {
		return false
	}

	if query.MaxViewers != nil && stream.ViewerCount > *query.MaxViewers {
		return false
	}

	// Search in title and description
	if query.Search != "" {
		searchTerm := strings.ToLower(query.Search)
		titleMatch := strings.Contains(strings.ToLower(stream.Title), searchTerm)
		descMatch := strings.Contains(strings.ToLower(stream.Description), searchTerm)

		if !titleMatch && !descMatch {
			return false
		}
	}

	// Filter by metadata
	if len(query.Metadata) > 0 {
		for key, value := range query.Metadata {
			streamValue, exists := stream.Metadata[key]
			if !exists || streamValue != value {
				return false
			}
		}
	}

	return true
}

// sortStreams sorts streams by the specified field and order
func sortStreams(streams []*Stream, sortBy string, sortOrder string) {
	// Default to descending order
	ascending := sortOrder == "asc"

	sort.Slice(streams, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "created_at":
			less = streams[i].CreatedAt.Before(streams[j].CreatedAt)

		case "started_at":
			// Handle nil started times
			if streams[i].StartedAt == nil && streams[j].StartedAt == nil {
				less = streams[i].CreatedAt.Before(streams[j].CreatedAt)
			} else if streams[i].StartedAt == nil {
				less = false
			} else if streams[j].StartedAt == nil {
				less = true
			} else {
				less = streams[i].StartedAt.Before(*streams[j].StartedAt)
			}

		case "viewer_count":
			less = streams[i].ViewerCount < streams[j].ViewerCount

		case "duration":
			less = streams[i].GetDuration() < streams[j].GetDuration()

		case "title":
			less = streams[i].Title < streams[j].Title

		default:
			// Default to created_at
			less = streams[i].CreatedAt.Before(streams[j].CreatedAt)
		}

		if ascending {
			return less
		}
		return !less
	})
}

// GetLiveStreams returns all currently live streams
func (sm *StreamManager) GetLiveStreams(ctx context.Context) ([]*Stream, error) {
	query := NewStreamQueryBuilder().
		WithState(StateLive).
		SortBy("started_at").
		SortOrder("desc").
		Build()

	result, err := sm.QueryStreams(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Streams, nil
}

// GetRecentStreams returns recently created streams
func (sm *StreamManager) GetRecentStreams(ctx context.Context, limit int) ([]*Stream, error) {
	query := NewStreamQueryBuilder().
		SortBy("created_at").
		SortOrder("desc").
		Limit(limit).
		Build()

	result, err := sm.QueryStreams(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Streams, nil
}

// GetPopularStreams returns streams with most viewers
func (sm *StreamManager) GetPopularStreams(ctx context.Context, limit int) ([]*Stream, error) {
	query := NewStreamQueryBuilder().
		WithState(StateLive).
		SortBy("viewer_count").
		SortOrder("desc").
		Limit(limit).
		Build()

	result, err := sm.QueryStreams(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Streams, nil
}

// SearchStreams searches for streams by title or description
func (sm *StreamManager) SearchStreams(ctx context.Context, searchTerm string, limit int) ([]*Stream, error) {
	if searchTerm == "" {
		return nil, fmt.Errorf("search term is required")
	}

	query := NewStreamQueryBuilder().
		WithSearch(searchTerm).
		SortBy("created_at").
		SortOrder("desc").
		Limit(limit).
		Build()

	result, err := sm.QueryStreams(ctx, query)
	if err != nil {
		return nil, err
	}

	return result.Streams, nil
}
