package interactive

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// PollType represents the type of poll
type PollType string

const (
	// PollTypeSingleChoice allows users to select only one option
	PollTypeSingleChoice PollType = "single_choice"
	// PollTypeMultipleChoice allows users to select multiple options
	PollTypeMultipleChoice PollType = "multiple_choice"
)

// PollStatus represents the current status of a poll
type PollStatus string

const (
	// PollStatusActive indicates the poll is currently active and accepting votes
	PollStatusActive PollStatus = "active"
	// PollStatusClosed indicates the poll has been closed
	PollStatusClosed PollStatus = "closed"
	// PollStatusPaused indicates the poll is temporarily paused
	PollStatusPaused PollStatus = "paused"
)

// PollOption represents a single option in a poll
type PollOption struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	VoteCount  int     `json:"vote_count"`
	Percentage float64 `json:"percentage"`
	Color      string  `json:"color,omitempty"`     // Optional color for visualization
	ImageURL   string  `json:"image_url,omitempty"` // Optional image for the option
}

// Poll represents a poll that can be displayed during a livestream
type Poll struct {
	ID          string                     `json:"id"`
	StreamID    string                     `json:"stream_id"`
	Question    string                     `json:"question"`
	Type        PollType                   `json:"type"`
	Options     []*PollOption              `json:"options"`
	Status      PollStatus                 `json:"status"`
	CreatedAt   time.Time                  `json:"created_at"`
	StartedAt   *time.Time                 `json:"started_at,omitempty"`
	ClosedAt    *time.Time                 `json:"closed_at,omitempty"`
	Duration    time.Duration              `json:"duration,omitempty"` // Optional auto-close duration
	TotalVotes  int                        `json:"total_votes"`
	AllowRevote bool                       `json:"allow_revote"` // Whether users can change their vote
	mu          sync.RWMutex               `json:"-"`
	voters      map[string]map[string]bool `json:"-"` // userID -> optionIDs map
}

// Vote represents a user's vote on a poll
type Vote struct {
	PollID    string    `json:"poll_id"`
	UserID    string    `json:"user_id"`
	OptionIDs []string  `json:"option_ids"`
	VotedAt   time.Time `json:"voted_at"`
}

// PollManager manages polls for livestreams
type PollManager struct {
	polls       map[string]*Poll   // pollID -> Poll
	streamPolls map[string][]*Poll // streamID -> []*Poll
	mu          sync.RWMutex
	callbacks   PollCallbacks
}

// PollCallbacks defines callback functions for poll events
type PollCallbacks struct {
	OnPollCreated func(poll *Poll)
	OnPollStarted func(poll *Poll)
	OnPollClosed  func(poll *Poll)
	OnVoteCast    func(vote *Vote, poll *Poll)
}

// PollConfig represents configuration for creating a poll
type PollConfig struct {
	StreamID    string
	Question    string
	Type        PollType
	Options     []string
	Duration    time.Duration
	AllowRevote bool
	AutoStart   bool
}

// NewPollManager creates a new poll manager
func NewPollManager() *PollManager {
	return &PollManager{
		polls:       make(map[string]*Poll),
		streamPolls: make(map[string][]*Poll),
	}
}

// SetCallbacks sets the callback functions for poll events
func (pm *PollManager) SetCallbacks(callbacks PollCallbacks) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.callbacks = callbacks
}

// CreatePoll creates a new poll
func (pm *PollManager) CreatePoll(config PollConfig) (*Poll, error) {
	if config.Question == "" {
		return nil, errors.New("poll question cannot be empty")
	}
	if len(config.Options) < 2 {
		return nil, errors.New("poll must have at least 2 options")
	}
	if config.Type != PollTypeSingleChoice && config.Type != PollTypeMultipleChoice {
		return nil, fmt.Errorf("invalid poll type: %s", config.Type)
	}

	poll := &Poll{
		ID:          generatePollID(),
		StreamID:    config.StreamID,
		Question:    config.Question,
		Type:        config.Type,
		Options:     make([]*PollOption, len(config.Options)),
		Status:      PollStatusPaused, // Start as paused, activate with StartPoll
		CreatedAt:   time.Now(),
		Duration:    config.Duration,
		AllowRevote: config.AllowRevote,
		voters:      make(map[string]map[string]bool),
	}

	// Create poll options
	for i, optionText := range config.Options {
		poll.Options[i] = &PollOption{
			ID:         fmt.Sprintf("%s_opt_%d", poll.ID, i),
			Text:       optionText,
			VoteCount:  0,
			Percentage: 0.0,
		}
	}

	pm.mu.Lock()
	pm.polls[poll.ID] = poll
	pm.streamPolls[config.StreamID] = append(pm.streamPolls[config.StreamID], poll)
	pm.mu.Unlock()

	// Trigger callback
	if pm.callbacks.OnPollCreated != nil {
		pm.callbacks.OnPollCreated(poll)
	}

	// Auto-start if configured
	if config.AutoStart {
		return poll, pm.StartPoll(poll.ID)
	}

	return poll, nil
}

// StartPoll starts a poll
func (pm *PollManager) StartPoll(pollID string) error {
	pm.mu.Lock()
	poll, exists := pm.polls[pollID]
	pm.mu.Unlock()

	if !exists {
		return fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.Lock()
	defer poll.mu.Unlock()

	if poll.Status == PollStatusActive {
		return errors.New("poll is already active")
	}

	if poll.Status == PollStatusClosed {
		return errors.New("cannot start a closed poll")
	}

	now := time.Now()
	poll.Status = PollStatusActive
	poll.StartedAt = &now

	// Schedule auto-close if duration is set
	if poll.Duration > 0 {
		go func() {
			time.Sleep(poll.Duration)
			pm.ClosePoll(pollID)
		}()
	}

	// Trigger callback
	if pm.callbacks.OnPollStarted != nil {
		pm.callbacks.OnPollStarted(poll)
	}

	return nil
}

// Vote casts a vote on a poll
func (pm *PollManager) Vote(pollID, userID string, optionIDs []string) error {
	pm.mu.RLock()
	poll, exists := pm.polls[pollID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.Lock()
	defer poll.mu.Unlock()

	if poll.Status != PollStatusActive {
		return fmt.Errorf("poll is not active: %s", poll.Status)
	}

	if len(optionIDs) == 0 {
		return errors.New("must select at least one option")
	}

	// Validate option IDs
	validOptions := make(map[string]bool)
	for _, opt := range poll.Options {
		validOptions[opt.ID] = true
	}

	for _, optID := range optionIDs {
		if !validOptions[optID] {
			return fmt.Errorf("invalid option ID: %s", optID)
		}
	}

	// Check poll type constraints
	if poll.Type == PollTypeSingleChoice && len(optionIDs) > 1 {
		return errors.New("can only select one option for single choice poll")
	}

	// Check if user has already voted
	existingVote, hasVoted := poll.voters[userID]
	if hasVoted && !poll.AllowRevote {
		return errors.New("user has already voted and revoting is not allowed")
	}

	// Remove previous votes if revoting
	if hasVoted {
		for optID := range existingVote {
			for _, opt := range poll.Options {
				if opt.ID == optID {
					opt.VoteCount--
					poll.TotalVotes--
					break
				}
			}
		}
	}

	// Record new votes
	poll.voters[userID] = make(map[string]bool)
	for _, optID := range optionIDs {
		poll.voters[userID][optID] = true
		for _, opt := range poll.Options {
			if opt.ID == optID {
				opt.VoteCount++
				poll.TotalVotes++
				break
			}
		}
	}

	// Update percentages
	pm.updatePercentages(poll)

	vote := &Vote{
		PollID:    pollID,
		UserID:    userID,
		OptionIDs: optionIDs,
		VotedAt:   time.Now(),
	}

	// Trigger callback
	if pm.callbacks.OnVoteCast != nil {
		pm.callbacks.OnVoteCast(vote, poll)
	}

	return nil
}

// updatePercentages updates the percentage for each option
func (pm *PollManager) updatePercentages(poll *Poll) {
	if poll.TotalVotes == 0 {
		for _, opt := range poll.Options {
			opt.Percentage = 0.0
		}
		return
	}

	for _, opt := range poll.Options {
		opt.Percentage = float64(opt.VoteCount) / float64(poll.TotalVotes) * 100.0
	}
}

// ClosePoll closes a poll
func (pm *PollManager) ClosePoll(pollID string) error {
	pm.mu.RLock()
	poll, exists := pm.polls[pollID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.Lock()
	defer poll.mu.Unlock()

	if poll.Status == PollStatusClosed {
		return errors.New("poll is already closed")
	}

	now := time.Now()
	poll.Status = PollStatusClosed
	poll.ClosedAt = &now

	// Trigger callback
	if pm.callbacks.OnPollClosed != nil {
		pm.callbacks.OnPollClosed(poll)
	}

	return nil
}

// PausePoll pauses an active poll
func (pm *PollManager) PausePoll(pollID string) error {
	pm.mu.RLock()
	poll, exists := pm.polls[pollID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.Lock()
	defer poll.mu.Unlock()

	if poll.Status != PollStatusActive {
		return fmt.Errorf("can only pause active polls, current status: %s", poll.Status)
	}

	poll.Status = PollStatusPaused
	return nil
}

// ResumePoll resumes a paused poll
func (pm *PollManager) ResumePoll(pollID string) error {
	pm.mu.RLock()
	poll, exists := pm.polls[pollID]
	pm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.Lock()
	defer poll.mu.Unlock()

	if poll.Status != PollStatusPaused {
		return fmt.Errorf("can only resume paused polls, current status: %s", poll.Status)
	}

	poll.Status = PollStatusActive
	return nil
}

// GetPoll returns a poll by ID
func (pm *PollManager) GetPoll(pollID string) (*Poll, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	poll, exists := pm.polls[pollID]
	if !exists {
		return nil, fmt.Errorf("poll not found: %s", pollID)
	}

	return poll, nil
}

// GetStreamPolls returns all polls for a stream
func (pm *PollManager) GetStreamPolls(streamID string) []*Poll {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	polls := pm.streamPolls[streamID]
	result := make([]*Poll, len(polls))
	copy(result, polls)
	return result
}

// GetActivePoll returns the currently active poll for a stream
func (pm *PollManager) GetActivePoll(streamID string) *Poll {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	polls := pm.streamPolls[streamID]
	for i := len(polls) - 1; i >= 0; i-- {
		if polls[i].Status == PollStatusActive {
			return polls[i]
		}
	}
	return nil
}

// GetResults returns the results of a poll
func (pm *PollManager) GetResults(pollID string) (*PollResults, error) {
	pm.mu.RLock()
	poll, exists := pm.polls[pollID]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.RLock()
	defer poll.mu.RUnlock()

	results := &PollResults{
		PollID:     poll.ID,
		Question:   poll.Question,
		Type:       poll.Type,
		Status:     poll.Status,
		TotalVotes: poll.TotalVotes,
		Options:    make([]*PollOptionResult, len(poll.Options)),
		CreatedAt:  poll.CreatedAt,
		ClosedAt:   poll.ClosedAt,
	}

	for i, opt := range poll.Options {
		results.Options[i] = &PollOptionResult{
			ID:         opt.ID,
			Text:       opt.Text,
			VoteCount:  opt.VoteCount,
			Percentage: opt.Percentage,
		}
	}

	// Find winning option(s)
	maxVotes := 0
	for _, opt := range results.Options {
		if opt.VoteCount > maxVotes {
			maxVotes = opt.VoteCount
		}
	}
	for _, opt := range results.Options {
		if opt.VoteCount == maxVotes && maxVotes > 0 {
			results.WinningOptions = append(results.WinningOptions, opt.ID)
		}
	}

	return results, nil
}

// PollResults represents the results of a poll
type PollResults struct {
	PollID         string              `json:"poll_id"`
	Question       string              `json:"question"`
	Type           PollType            `json:"type"`
	Status         PollStatus          `json:"status"`
	TotalVotes     int                 `json:"total_votes"`
	Options        []*PollOptionResult `json:"options"`
	WinningOptions []string            `json:"winning_options"`
	CreatedAt      time.Time           `json:"created_at"`
	ClosedAt       *time.Time          `json:"closed_at,omitempty"`
}

// PollOptionResult represents a poll option with results
type PollOptionResult struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	VoteCount  int     `json:"vote_count"`
	Percentage float64 `json:"percentage"`
}

// DeletePoll deletes a poll
func (pm *PollManager) DeletePoll(pollID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	poll, exists := pm.polls[pollID]
	if !exists {
		return fmt.Errorf("poll not found: %s", pollID)
	}

	// Remove from stream polls
	streamPolls := pm.streamPolls[poll.StreamID]
	for i, p := range streamPolls {
		if p.ID == pollID {
			pm.streamPolls[poll.StreamID] = append(streamPolls[:i], streamPolls[i+1:]...)
			break
		}
	}

	// Remove from polls map
	delete(pm.polls, pollID)

	return nil
}

// GetUserVote returns the user's vote for a poll
func (pm *PollManager) GetUserVote(pollID, userID string) ([]string, error) {
	pm.mu.RLock()
	poll, exists := pm.polls[pollID]
	pm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("poll not found: %s", pollID)
	}

	poll.mu.RLock()
	defer poll.mu.RUnlock()

	vote, hasVoted := poll.voters[userID]
	if !hasVoted {
		return nil, nil
	}

	optionIDs := make([]string, 0, len(vote))
	for optID := range vote {
		optionIDs = append(optionIDs, optID)
	}

	return optionIDs, nil
}

// generatePollID generates a unique poll ID
func generatePollID() string {
	return fmt.Sprintf("poll_%d", time.Now().UnixNano())
}
