package interactive

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// GiftType represents the type of gift
type GiftType string

const (
	// GiftTypeStatic represents a static image gift
	GiftTypeStatic GiftType = "static"
	// GiftTypeAnimated represents an animated gift
	GiftTypeAnimated GiftType = "animated"
	// GiftTypeCombo represents a combo gift (multiple items)
	GiftTypeCombo GiftType = "combo"
	// GiftTypeSpecial represents a special effect gift
	GiftTypeSpecial GiftType = "special"
)

// GiftRarity represents the rarity level of a gift
type GiftRarity string

const (
	// GiftRarityCommon represents common gifts
	GiftRarityCommon GiftRarity = "common"
	// GiftRarityRare represents rare gifts
	GiftRarityRare GiftRarity = "rare"
	// GiftRarityEpic represents epic gifts
	GiftRarityEpic GiftRarity = "epic"
	// GiftRarityLegendary represents legendary gifts
	GiftRarityLegendary GiftRarity = "legendary"
)

// Gift represents a virtual gift that can be sent during a livestream
type Gift struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Type         GiftType               `json:"type"`
	Rarity       GiftRarity             `json:"rarity"`
	Price        int64                  `json:"price"`
	Currency     CurrencyType           `json:"currency"`
	ImageURL     string                 `json:"image_url"`
	AnimationURL string                 `json:"animation_url,omitempty"`
	Duration     time.Duration          `json:"duration,omitempty"` // Animation duration
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	IsAvailable  bool                   `json:"is_available"`
	CreatedAt    time.Time              `json:"created_at"`
}

// GiftSent represents a gift that was sent during a stream
type GiftSent struct {
	ID         string            `json:"id"`
	GiftID     string            `json:"gift_id"`
	StreamID   string            `json:"stream_id"`
	FromUserID string            `json:"from_user_id"`
	ToUserID   string            `json:"to_user_id"` // Streamer or other user
	Amount     int               `json:"amount"`     // Number of gifts (for combo)
	TotalValue int64             `json:"total_value"`
	Message    string            `json:"message,omitempty"`
	SentAt     time.Time         `json:"sent_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// GiftCatalog manages the catalog of available gifts
type GiftCatalog struct {
	gifts map[string]*Gift // giftID -> Gift
	mu    sync.RWMutex
}

// GiftManager manages gift sending and receiving
type GiftManager struct {
	catalog       *GiftCatalog
	currencyMgr   *CurrencyManager
	sentGifts     map[string]*GiftSent   // sentGiftID -> GiftSent
	streamGifts   map[string][]*GiftSent // streamID -> []*GiftSent
	userGiftsSent map[string][]*GiftSent // userID -> []*GiftSent (gifts sent by user)
	userGiftsRecv map[string][]*GiftSent // userID -> []*GiftSent (gifts received by user)
	mu            sync.RWMutex
	callbacks     GiftCallbacks
}

// GiftCallbacks defines callback functions for gift events
type GiftCallbacks struct {
	OnGiftSent     func(giftSent *GiftSent, gift *Gift)
	OnGiftReceived func(giftSent *GiftSent, gift *Gift)
	OnComboUpdate  func(streamID, userID, giftID string, comboCount int)
}

// NewGiftCatalog creates a new gift catalog
func NewGiftCatalog() *GiftCatalog {
	return &GiftCatalog{
		gifts: make(map[string]*Gift),
	}
}

// AddGift adds a gift to the catalog
func (gc *GiftCatalog) AddGift(gift *Gift) error {
	if gift.ID == "" {
		return errors.New("gift ID cannot be empty")
	}
	if gift.Name == "" {
		return errors.New("gift name cannot be empty")
	}
	if gift.Price < 0 {
		return errors.New("gift price cannot be negative")
	}

	gc.mu.Lock()
	defer gc.mu.Unlock()

	if _, exists := gc.gifts[gift.ID]; exists {
		return fmt.Errorf("gift already exists: %s", gift.ID)
	}

	gift.CreatedAt = time.Now()
	gc.gifts[gift.ID] = gift

	return nil
}

// GetGift returns a gift by ID
func (gc *GiftCatalog) GetGift(giftID string) (*Gift, error) {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	gift, exists := gc.gifts[giftID]
	if !exists {
		return nil, fmt.Errorf("gift not found: %s", giftID)
	}

	return gift, nil
}

// GetAllGifts returns all available gifts
func (gc *GiftCatalog) GetAllGifts() []*Gift {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	gifts := make([]*Gift, 0, len(gc.gifts))
	for _, gift := range gc.gifts {
		if gift.IsAvailable {
			gifts = append(gifts, gift)
		}
	}

	return gifts
}

// GetGiftsByRarity returns gifts of a specific rarity
func (gc *GiftCatalog) GetGiftsByRarity(rarity GiftRarity) []*Gift {
	gc.mu.RLock()
	defer gc.mu.RUnlock()

	gifts := make([]*Gift, 0)
	for _, gift := range gc.gifts {
		if gift.IsAvailable && gift.Rarity == rarity {
			gifts = append(gifts, gift)
		}
	}

	return gifts
}

// UpdateGift updates an existing gift
func (gc *GiftCatalog) UpdateGift(giftID string, updates map[string]interface{}) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	gift, exists := gc.gifts[giftID]
	if !exists {
		return fmt.Errorf("gift not found: %s", giftID)
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		gift.Name = name
	}
	if description, ok := updates["description"].(string); ok {
		gift.Description = description
	}
	if price, ok := updates["price"].(int64); ok {
		gift.Price = price
	}
	if isAvailable, ok := updates["is_available"].(bool); ok {
		gift.IsAvailable = isAvailable
	}

	return nil
}

// DeleteGift removes a gift from the catalog
func (gc *GiftCatalog) DeleteGift(giftID string) error {
	gc.mu.Lock()
	defer gc.mu.Unlock()

	if _, exists := gc.gifts[giftID]; !exists {
		return fmt.Errorf("gift not found: %s", giftID)
	}

	delete(gc.gifts, giftID)
	return nil
}

// NewGiftManager creates a new gift manager
func NewGiftManager(catalog *GiftCatalog, currencyMgr *CurrencyManager) *GiftManager {
	return &GiftManager{
		catalog:       catalog,
		currencyMgr:   currencyMgr,
		sentGifts:     make(map[string]*GiftSent),
		streamGifts:   make(map[string][]*GiftSent),
		userGiftsSent: make(map[string][]*GiftSent),
		userGiftsRecv: make(map[string][]*GiftSent),
	}
}

// SetCallbacks sets the callback functions for gift events
func (gm *GiftManager) SetCallbacks(callbacks GiftCallbacks) {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.callbacks = callbacks
}

// SendGift sends a gift from one user to another
func (gm *GiftManager) SendGift(streamID, giftID, fromUserID, toUserID string, amount int, message string) (*GiftSent, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	// Get gift from catalog
	gift, err := gm.catalog.GetGift(giftID)
	if err != nil {
		return nil, err
	}

	if !gift.IsAvailable {
		return nil, errors.New("gift is not available")
	}

	// Calculate total cost
	totalCost := gift.Price * int64(amount)

	// Deduct currency from sender
	txn, err := gm.currencyMgr.DeductBalance(
		fromUserID,
		gift.Currency,
		totalCost,
		fmt.Sprintf("Sent %d x %s to %s", amount, gift.Name, toUserID),
		map[string]string{
			"gift_id":   giftID,
			"stream_id": streamID,
			"to_user":   toUserID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to deduct balance: %w", err)
	}

	// Create gift sent record
	giftSent := &GiftSent{
		ID:         generateGiftSentID(),
		GiftID:     giftID,
		StreamID:   streamID,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		Amount:     amount,
		TotalValue: totalCost,
		Message:    message,
		SentAt:     time.Now(),
		Metadata: map[string]string{
			"transaction_id": txn.ID,
		},
	}

	gm.mu.Lock()
	gm.sentGifts[giftSent.ID] = giftSent
	gm.streamGifts[streamID] = append(gm.streamGifts[streamID], giftSent)
	gm.userGiftsSent[fromUserID] = append(gm.userGiftsSent[fromUserID], giftSent)
	gm.userGiftsRecv[toUserID] = append(gm.userGiftsRecv[toUserID], giftSent)
	gm.mu.Unlock()

	// Trigger callbacks
	if gm.callbacks.OnGiftSent != nil {
		gm.callbacks.OnGiftSent(giftSent, gift)
	}
	if gm.callbacks.OnGiftReceived != nil {
		gm.callbacks.OnGiftReceived(giftSent, gift)
	}

	// Check for combo
	gm.checkCombo(streamID, fromUserID, giftID)

	return giftSent, nil
}

// checkCombo checks if the user is sending gifts in a combo
func (gm *GiftManager) checkCombo(streamID, userID, giftID string) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	streamGifts := gm.streamGifts[streamID]
	if len(streamGifts) == 0 {
		return
	}

	// Check last 10 seconds for combos
	comboWindow := 10 * time.Second
	now := time.Now()
	comboCount := 0

	// Count gifts of same type sent by same user in the combo window
	for i := len(streamGifts) - 1; i >= 0; i-- {
		gift := streamGifts[i]
		if now.Sub(gift.SentAt) > comboWindow {
			break
		}
		if gift.FromUserID == userID && gift.GiftID == giftID {
			comboCount += gift.Amount
		}
	}

	// Trigger combo callback if threshold met (e.g., 3+ gifts)
	if comboCount >= 3 && gm.callbacks.OnComboUpdate != nil {
		gm.callbacks.OnComboUpdate(streamID, userID, giftID, comboCount)
	}
}

// GetStreamGifts returns all gifts sent in a stream
func (gm *GiftManager) GetStreamGifts(streamID string) []*GiftSent {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	gifts := gm.streamGifts[streamID]
	result := make([]*GiftSent, len(gifts))
	copy(result, gifts)
	return result
}

// GetUserSentGifts returns gifts sent by a user
func (gm *GiftManager) GetUserSentGifts(userID string, limit int) []*GiftSent {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	gifts := gm.userGiftsSent[userID]
	if len(gifts) == 0 {
		return []*GiftSent{}
	}

	start := len(gifts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*GiftSent, 0, limit)
	for i := len(gifts) - 1; i >= start; i-- {
		result = append(result, gifts[i])
	}

	return result
}

// GetUserReceivedGifts returns gifts received by a user
func (gm *GiftManager) GetUserReceivedGifts(userID string, limit int) []*GiftSent {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	gifts := gm.userGiftsRecv[userID]
	if len(gifts) == 0 {
		return []*GiftSent{}
	}

	start := len(gifts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*GiftSent, 0, limit)
	for i := len(gifts) - 1; i >= start; i-- {
		result = append(result, gifts[i])
	}

	return result
}

// GetStreamGiftStats returns statistics for gifts in a stream
func (gm *GiftManager) GetStreamGiftStats(streamID string) *GiftStats {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	gifts := gm.streamGifts[streamID]
	stats := &GiftStats{
		StreamID:      streamID,
		TotalGifts:    len(gifts),
		TotalValue:    0,
		GiftCountByID: make(map[string]int),
		ValueByID:     make(map[string]int64),
		TopSenders:    make(map[string]int64),
		TopReceivers:  make(map[string]int64),
	}

	for _, gift := range gifts {
		stats.TotalValue += gift.TotalValue
		stats.GiftCountByID[gift.GiftID] += gift.Amount
		stats.ValueByID[gift.GiftID] += gift.TotalValue
		stats.TopSenders[gift.FromUserID] += gift.TotalValue
		stats.TopReceivers[gift.ToUserID] += gift.TotalValue
	}

	return stats
}

// GetUserGiftStats returns statistics for a user's gifting activity
func (gm *GiftManager) GetUserGiftStats(userID string) *UserGiftStats {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	sentGifts := gm.userGiftsSent[userID]
	recvGifts := gm.userGiftsRecv[userID]

	stats := &UserGiftStats{
		UserID:             userID,
		TotalGiftsSent:     len(sentGifts),
		TotalGiftsReceived: len(recvGifts),
		TotalValueSent:     0,
		TotalValueReceived: 0,
		FavoriteGifts:      make(map[string]int),
	}

	for _, gift := range sentGifts {
		stats.TotalValueSent += gift.TotalValue
		stats.FavoriteGifts[gift.GiftID] += gift.Amount
	}

	for _, gift := range recvGifts {
		stats.TotalValueReceived += gift.TotalValue
	}

	return stats
}

// GiftStats represents statistics for gifts in a stream
type GiftStats struct {
	StreamID      string           `json:"stream_id"`
	TotalGifts    int              `json:"total_gifts"`
	TotalValue    int64            `json:"total_value"`
	GiftCountByID map[string]int   `json:"gift_count_by_id"`
	ValueByID     map[string]int64 `json:"value_by_id"`
	TopSenders    map[string]int64 `json:"top_senders"`
	TopReceivers  map[string]int64 `json:"top_receivers"`
}

// UserGiftStats represents statistics for a user's gifting activity
type UserGiftStats struct {
	UserID             string         `json:"user_id"`
	TotalGiftsSent     int            `json:"total_gifts_sent"`
	TotalGiftsReceived int            `json:"total_gifts_received"`
	TotalValueSent     int64          `json:"total_value_sent"`
	TotalValueReceived int64          `json:"total_value_received"`
	FavoriteGifts      map[string]int `json:"favorite_gifts"`
}

// GiftLeaderboard returns top gift senders for a stream
func (gm *GiftManager) GetGiftLeaderboard(streamID string, limit int) []*LeaderboardEntry {
	stats := gm.GetStreamGiftStats(streamID)

	entries := make([]*LeaderboardEntry, 0, len(stats.TopSenders))
	for userID, value := range stats.TopSenders {
		entries = append(entries, &LeaderboardEntry{
			UserID:     userID,
			TotalValue: value,
			Rank:       0, // Will be set after sorting
		})
	}

	// Sort by total value (descending)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].TotalValue > entries[i].TotalValue {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Assign ranks and limit results
	if len(entries) > limit {
		entries = entries[:limit]
	}
	for i := range entries {
		entries[i].Rank = i + 1
	}

	return entries
}

// LeaderboardEntry represents an entry in the gift leaderboard
type LeaderboardEntry struct {
	UserID     string `json:"user_id"`
	TotalValue int64  `json:"total_value"`
	Rank       int    `json:"rank"`
}

// GetRecentGifts returns the most recent gifts sent in a stream
func (gm *GiftManager) GetRecentGifts(streamID string, limit int) []*GiftSent {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	gifts := gm.streamGifts[streamID]
	if len(gifts) == 0 {
		return []*GiftSent{}
	}

	start := len(gifts) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*GiftSent, 0, limit)
	for i := len(gifts) - 1; i >= start; i-- {
		result = append(result, gifts[i])
	}

	return result
}

// generateGiftSentID generates a unique gift sent ID
func generateGiftSentID() string {
	return fmt.Sprintf("gift_sent_%d", time.Now().UnixNano())
}
