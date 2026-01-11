package interactive

import (
	"testing"
	"time"
)

func TestPollManager_CreatePoll(t *testing.T) {
	pm := NewPollManager()

	config := PollConfig{
		StreamID:  "stream1",
		Question:  "What's your favorite color?",
		Type:      PollTypeSingleChoice,
		Options:   []string{"Red", "Blue", "Green", "Yellow"},
		AutoStart: false, // Don't auto-start to avoid race conditions
	}

	poll, err := pm.CreatePoll(config)
	if err != nil {
		t.Fatalf("Failed to create poll: %v", err)
	}

	if poll.Question != config.Question {
		t.Errorf("Expected question %s, got %s", config.Question, poll.Question)
	}

	if len(poll.Options) != 4 {
		t.Errorf("Expected 4 options, got %d", len(poll.Options))
	}

	// Start the poll
	err = pm.StartPoll(poll.ID)
	if err != nil {
		t.Fatalf("Failed to start poll: %v", err)
	}

	if poll.Status != PollStatusActive {
		t.Errorf("Expected status %s, got %s", PollStatusActive, poll.Status)
	}
}

func TestPollManager_Vote(t *testing.T) {
	pm := NewPollManager()

	poll, _ := pm.CreatePoll(PollConfig{
		StreamID:  "stream1",
		Question:  "Test poll",
		Type:      PollTypeSingleChoice,
		Options:   []string{"Option 1", "Option 2"},
		AutoStart: true,
	})

	// Cast vote
	err := pm.Vote(poll.ID, "user1", []string{poll.Options[0].ID})
	if err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	// Check vote count
	if poll.Options[0].VoteCount != 1 {
		t.Errorf("Expected vote count 1, got %d", poll.Options[0].VoteCount)
	}

	if poll.TotalVotes != 1 {
		t.Errorf("Expected total votes 1, got %d", poll.TotalVotes)
	}

	// Test duplicate vote (should fail if revote not allowed)
	err = pm.Vote(poll.ID, "user1", []string{poll.Options[1].ID})
	if err == nil {
		t.Error("Expected error for duplicate vote without revote enabled")
	}
}

func TestPollManager_MultipleChoice(t *testing.T) {
	pm := NewPollManager()

	poll, _ := pm.CreatePoll(PollConfig{
		StreamID:  "stream1",
		Question:  "Select your favorites",
		Type:      PollTypeMultipleChoice,
		Options:   []string{"A", "B", "C"},
		AutoStart: true,
	})

	// Vote for multiple options
	err := pm.Vote(poll.ID, "user1", []string{poll.Options[0].ID, poll.Options[2].ID})
	if err != nil {
		t.Fatalf("Failed to vote: %v", err)
	}

	if poll.Options[0].VoteCount != 1 || poll.Options[2].VoteCount != 1 {
		t.Error("Vote counts not updated correctly for multiple choice")
	}

	if poll.TotalVotes != 2 {
		t.Errorf("Expected total votes 2, got %d", poll.TotalVotes)
	}
}

func TestPollManager_GetResults(t *testing.T) {
	pm := NewPollManager()

	poll, _ := pm.CreatePoll(PollConfig{
		StreamID:  "stream1",
		Question:  "Test poll",
		Type:      PollTypeSingleChoice,
		Options:   []string{"Winner", "Loser"},
		AutoStart: true,
	})

	// Cast votes
	pm.Vote(poll.ID, "user1", []string{poll.Options[0].ID})
	pm.Vote(poll.ID, "user2", []string{poll.Options[0].ID})
	pm.Vote(poll.ID, "user3", []string{poll.Options[1].ID})

	results, err := pm.GetResults(poll.ID)
	if err != nil {
		t.Fatalf("Failed to get results: %v", err)
	}

	if results.TotalVotes != 3 {
		t.Errorf("Expected 3 total votes, got %d", results.TotalVotes)
	}

	if len(results.WinningOptions) != 1 {
		t.Errorf("Expected 1 winning option, got %d", len(results.WinningOptions))
	}

	if results.WinningOptions[0] != poll.Options[0].ID {
		t.Error("Wrong winning option")
	}
}

func TestCurrencyManager_Balance(t *testing.T) {
	cm := NewCurrencyManager()

	// Check initial balance
	balance, _ := cm.GetBalance("user1", CurrencyTypeCoins)
	if balance != 0 {
		t.Errorf("Expected initial balance 0, got %d", balance)
	}

	// Add balance
	_, err := cm.AddBalance("user1", CurrencyTypeCoins, 100, "Initial coins", nil)
	if err != nil {
		t.Fatalf("Failed to add balance: %v", err)
	}

	balance, _ = cm.GetBalance("user1", CurrencyTypeCoins)
	if balance != 100 {
		t.Errorf("Expected balance 100, got %d", balance)
	}

	// Deduct balance
	_, err = cm.DeductBalance("user1", CurrencyTypeCoins, 30, "Purchase", nil)
	if err != nil {
		t.Fatalf("Failed to deduct balance: %v", err)
	}

	balance, _ = cm.GetBalance("user1", CurrencyTypeCoins)
	if balance != 70 {
		t.Errorf("Expected balance 70, got %d", balance)
	}
}

func TestCurrencyManager_InsufficientBalance(t *testing.T) {
	cm := NewCurrencyManager()

	cm.AddBalance("user1", CurrencyTypeCoins, 50, "Test", nil)

	_, err := cm.DeductBalance("user1", CurrencyTypeCoins, 100, "Purchase", nil)
	if err == nil {
		t.Error("Expected error for insufficient balance")
	}
}

func TestCurrencyManager_Transfer(t *testing.T) {
	cm := NewCurrencyManager()

	// Setup balances
	cm.AddBalance("user1", CurrencyTypeCoins, 100, "Initial", nil)

	// Transfer
	_, _, err := cm.Transfer("user1", "user2", CurrencyTypeCoins, 30, "Gift")
	if err != nil {
		t.Fatalf("Failed to transfer: %v", err)
	}

	// Check balances
	balance1, _ := cm.GetBalance("user1", CurrencyTypeCoins)
	balance2, _ := cm.GetBalance("user2", CurrencyTypeCoins)

	if balance1 != 70 {
		t.Errorf("Expected user1 balance 70, got %d", balance1)
	}

	if balance2 != 30 {
		t.Errorf("Expected user2 balance 30, got %d", balance2)
	}
}

func TestGiftManager_SendGift(t *testing.T) {
	catalog := NewGiftCatalog()
	cm := NewCurrencyManager()
	gm := NewGiftManager(catalog, cm)

	// Add gift to catalog
	gift := &Gift{
		ID:          "gift1",
		Name:        "Rose",
		Price:       10,
		Currency:    CurrencyTypeCoins,
		IsAvailable: true,
	}
	catalog.AddGift(gift)

	// Give user some coins
	cm.AddBalance("user1", CurrencyTypeCoins, 100, "Initial", nil)

	// Send gift
	giftSent, err := gm.SendGift("stream1", "gift1", "user1", "streamer1", 2, "Great stream!")
	if err != nil {
		t.Fatalf("Failed to send gift: %v", err)
	}

	if giftSent.Amount != 2 {
		t.Errorf("Expected 2 gifts, got %d", giftSent.Amount)
	}

	if giftSent.TotalValue != 20 {
		t.Errorf("Expected total value 20, got %d", giftSent.TotalValue)
	}

	// Check balance deducted
	balance, _ := cm.GetBalance("user1", CurrencyTypeCoins)
	if balance != 80 {
		t.Errorf("Expected balance 80, got %d", balance)
	}
}

func TestGiftManager_InsufficientFunds(t *testing.T) {
	catalog := NewGiftCatalog()
	cm := NewCurrencyManager()
	gm := NewGiftManager(catalog, cm)

	gift := &Gift{
		ID:          "gift1",
		Name:        "Diamond",
		Price:       100,
		Currency:    CurrencyTypeCoins,
		IsAvailable: true,
	}
	catalog.AddGift(gift)

	cm.AddBalance("user1", CurrencyTypeCoins, 50, "Initial", nil)

	_, err := gm.SendGift("stream1", "gift1", "user1", "streamer1", 1, "")
	if err == nil {
		t.Error("Expected error for insufficient funds")
	}
}

func TestReactionManager_AddReaction(t *testing.T) {
	rm := NewReactionManager()

	reaction, err := rm.AddReaction("stream1", "user1", ReactionTypeLike, "")
	if err != nil {
		t.Fatalf("Failed to add reaction: %v", err)
	}

	if reaction.Type != ReactionTypeLike {
		t.Errorf("Expected reaction type %s, got %s", ReactionTypeLike, reaction.Type)
	}

	// Get aggregate
	aggregate, _ := rm.GetStreamAggregate("stream1")
	if aggregate.TotalCount != 1 {
		t.Errorf("Expected 1 reaction, got %d", aggregate.TotalCount)
	}

	if aggregate.CountByType[ReactionTypeLike] != 1 {
		t.Error("Reaction count by type not updated")
	}
}

func TestReactionManager_BurstDetection(t *testing.T) {
	rm := NewReactionManager()
	rm.SetBurstThreshold(3, 2*time.Second)

	burstDetected := false
	rm.SetCallbacks(ReactionCallbacks{
		OnReactionBurst: func(streamID string, burst *ReactionBurst) {
			burstDetected = true
			if burst.Count < 3 {
				t.Errorf("Expected burst count >= 3, got %d", burst.Count)
			}
		},
	})

	// Add multiple reactions quickly
	for i := 0; i < 5; i++ {
		rm.AddReaction("stream1", "user1", ReactionTypeFire, "")
	}

	if !burstDetected {
		t.Error("Expected burst to be detected")
	}
}

func TestReactionManager_GetStats(t *testing.T) {
	rm := NewReactionManager()

	// Add various reactions
	rm.AddReaction("stream1", "user1", ReactionTypeLike, "")
	rm.AddReaction("stream1", "user2", ReactionTypeLike, "")
	rm.AddReaction("stream1", "user3", ReactionTypeLove, "")

	stats := rm.GetReactionStats("stream1", 10*time.Second)

	if stats.TotalReactions != 3 {
		t.Errorf("Expected 3 reactions, got %d", stats.TotalReactions)
	}

	if stats.ByType[ReactionTypeLike] != 2 {
		t.Errorf("Expected 2 like reactions, got %d", stats.ByType[ReactionTypeLike])
	}

	if len(stats.ByUser) != 3 {
		t.Errorf("Expected 3 users, got %d", len(stats.ByUser))
	}
}

func TestReactionManager_CleanupOld(t *testing.T) {
	rm := NewReactionManager()

	// Add reaction
	rm.AddReaction("stream1", "user1", ReactionTypeLike, "")

	// Sleep and add another
	time.Sleep(50 * time.Millisecond)
	rm.AddReaction("stream1", "user2", ReactionTypeLove, "")

	// Cleanup old reactions (older than 40ms)
	cleaned := rm.CleanupOldReactions(40 * time.Millisecond)

	if cleaned != 1 {
		t.Errorf("Expected 1 reaction cleaned, got %d", cleaned)
	}

	// Check aggregate
	aggregate, _ := rm.GetStreamAggregate("stream1")
	if aggregate.TotalCount != 2 {
		t.Error("Aggregate should still show total count of 2 (not decremented)")
	}
}
