package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aminofox/zenlive/pkg/interactive"
	"github.com/aminofox/zenlive/pkg/streaming"
)

func main() {
	fmt.Println("=== ZenLive Phase 11: Advanced Features Demo ===\n")

	// Run all examples
	runPollExample()
	runCurrencyExample()
	runGiftExample()
	runReactionExample()
	runMultiStreamExample()

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Press Ctrl+C to exit...")

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\nShutting down...")
}

func runPollExample() {
	fmt.Println("\n--- Poll System Example ---")

	pm := interactive.NewPollManager()

	// Set callbacks
	pm.SetCallbacks(interactive.PollCallbacks{
		OnPollCreated: func(poll *interactive.Poll) {
			fmt.Printf("✓ Poll created: %s\n", poll.Question)
		},
		OnPollStarted: func(poll *interactive.Poll) {
			fmt.Printf("✓ Poll started: %s\n", poll.Question)
		},
		OnVoteCast: func(vote *interactive.Vote, poll *interactive.Poll) {
			fmt.Printf("  → User %s voted on poll: %s\n", vote.UserID, poll.ID)
		},
		OnPollClosed: func(poll *interactive.Poll) {
			fmt.Printf("✓ Poll closed: %s (Total votes: %d)\n", poll.Question, poll.TotalVotes)
		},
	})

	// Create a single choice poll
	pollConfig := interactive.PollConfig{
		StreamID:    "stream1",
		Question:    "Which streaming protocol do you prefer?",
		Type:        interactive.PollTypeSingleChoice,
		Options:     []string{"RTMP", "HLS", "WebRTC", "DASH"},
		Duration:    10 * time.Second,
		AllowRevote: false,
		AutoStart:   false,
	}

	poll, err := pm.CreatePoll(pollConfig)
	if err != nil {
		fmt.Printf("Error creating poll: %v\n", err)
		return
	}

	// Start the poll
	pm.StartPoll(poll.ID)

	// Simulate votes
	votes := map[string]int{
		"RTMP":   5,
		"HLS":    12,
		"WebRTC": 8,
		"DASH":   3,
	}

	for i, option := range poll.Options {
		voteCount := votes[option.Text]
		for j := 0; j < voteCount; j++ {
			userID := fmt.Sprintf("user_%d_%d", i, j)
			pm.Vote(poll.ID, userID, []string{option.ID})
		}
	}

	// Get results
	results, _ := pm.GetResults(poll.ID)
	fmt.Println("\nPoll Results:")
	for _, opt := range results.Options {
		fmt.Printf("  %s: %d votes (%.1f%%)\n", opt.Text, opt.VoteCount, opt.Percentage)
	}
	fmt.Printf("  Winner: %s\n", results.Options[1].Text) // HLS wins

	// Close poll
	pm.ClosePoll(poll.ID)

	// Create a multiple choice poll
	multiPollConfig := interactive.PollConfig{
		StreamID:    "stream1",
		Question:    "What features do you want next? (Select all that apply)",
		Type:        interactive.PollTypeMultipleChoice,
		Options:     []string{"Screen Sharing", "Virtual Backgrounds", "Filters", "Recording"},
		AllowRevote: true,
		AutoStart:   false,
	}

	multiPoll, _ := pm.CreatePoll(multiPollConfig)
	pm.StartPoll(multiPoll.ID)

	// Vote with multiple selections
	pm.Vote(multiPoll.ID, "user1", []string{multiPoll.Options[0].ID, multiPoll.Options[2].ID})
	pm.Vote(multiPoll.ID, "user2", []string{multiPoll.Options[1].ID, multiPoll.Options[3].ID})
	pm.Vote(multiPoll.ID, "user3", []string{multiPoll.Options[0].ID, multiPoll.Options[1].ID, multiPoll.Options[2].ID})

	multiResults, _ := pm.GetResults(multiPoll.ID)
	fmt.Println("\nMultiple Choice Poll Results:")
	for _, opt := range multiResults.Options {
		fmt.Printf("  %s: %d selections\n", opt.Text, opt.VoteCount)
	}
}

func runCurrencyExample() {
	fmt.Println("\n--- Virtual Currency Example ---")

	cm := interactive.NewCurrencyManager()

	// Set callbacks
	cm.SetCallbacks(interactive.CurrencyCallbacks{
		OnBalanceChanged: func(userID string, currencyType interactive.CurrencyType, oldBalance, newBalance int64) {
			change := newBalance - oldBalance
			if change > 0 {
				fmt.Printf("  → %s earned %d %s (Balance: %d)\n", userID, change, currencyType, newBalance)
			} else {
				fmt.Printf("  → %s spent %d %s (Balance: %d)\n", userID, -change, currencyType, newBalance)
			}
		},
		OnTransactionCreated: func(transaction *interactive.Transaction) {
			// Transaction logged
		},
	})

	// Give users initial balance
	fmt.Println("\nInitial Setup:")
	cm.AddBalance("viewer1", interactive.CurrencyTypeCoins, 100, "Welcome bonus", nil)
	cm.AddBalance("viewer2", interactive.CurrencyTypeCoins, 150, "Welcome bonus", nil)
	cm.AddBalance("streamer1", interactive.CurrencyTypeDiamonds, 50, "Creator reward", nil)

	// Purchase currency
	fmt.Println("\nPurchase:")
	cm.Purchase("viewer1", interactive.CurrencyTypeCoins, 200, 5.00, "Credit Card")

	// Transfer between users
	fmt.Println("\nTransfer:")
	cm.Transfer("viewer1", "viewer2", interactive.CurrencyTypeCoins, 50, "Gift from viewer1")

	// Check balances
	fmt.Println("\nFinal Balances:")
	balances1, _ := cm.GetAllBalances("viewer1")
	balances2, _ := cm.GetAllBalances("viewer2")
	streamerBalances, _ := cm.GetAllBalances("streamer1")

	fmt.Printf("  viewer1: Coins=%d, Diamonds=%d, Points=%d\n",
		balances1[interactive.CurrencyTypeCoins],
		balances1[interactive.CurrencyTypeDiamonds],
		balances1[interactive.CurrencyTypePoints])

	fmt.Printf("  viewer2: Coins=%d, Diamonds=%d, Points=%d\n",
		balances2[interactive.CurrencyTypeCoins],
		balances2[interactive.CurrencyTypeDiamonds],
		balances2[interactive.CurrencyTypePoints])

	fmt.Printf("  streamer1: Coins=%d, Diamonds=%d, Points=%d\n",
		streamerBalances[interactive.CurrencyTypeCoins],
		streamerBalances[interactive.CurrencyTypeDiamonds],
		streamerBalances[interactive.CurrencyTypePoints])

	// Show transaction history
	fmt.Println("\nviewer1 Transaction History:")
	txns, _ := cm.GetUserTransactions("viewer1", 5)
	for _, txn := range txns {
		fmt.Printf("  %s: %s %+d %s -> Balance: %d\n",
			txn.CreatedAt.Format("15:04:05"),
			txn.Type,
			txn.Amount,
			txn.CurrencyType,
			txn.Balance)
	}
}

func runGiftExample() {
	fmt.Println("\n--- Virtual Gift System Example ---")

	// Setup
	catalog := interactive.NewGiftCatalog()
	cm := interactive.NewCurrencyManager()
	gm := interactive.NewGiftManager(catalog, cm)

	// Add gifts to catalog
	gifts := []*interactive.Gift{
		{
			ID:          "rose",
			Name:        "Rose",
			Description: "A beautiful rose",
			Type:        interactive.GiftTypeStatic,
			Rarity:      interactive.GiftRarityCommon,
			Price:       10,
			Currency:    interactive.CurrencyTypeCoins,
			ImageURL:    "https://example.com/rose.png",
			IsAvailable: true,
		},
		{
			ID:           "heart",
			Name:         "Heart",
			Description:  "Send some love",
			Type:         interactive.GiftTypeAnimated,
			Rarity:       interactive.GiftRarityRare,
			Price:        50,
			Currency:     interactive.CurrencyTypeCoins,
			ImageURL:     "https://example.com/heart.png",
			AnimationURL: "https://example.com/heart.gif",
			Duration:     3 * time.Second,
			IsAvailable:  true,
		},
		{
			ID:           "diamond",
			Name:         "Diamond",
			Description:  "The ultimate gift",
			Type:         interactive.GiftTypeSpecial,
			Rarity:       interactive.GiftRarityLegendary,
			Price:        500,
			Currency:     interactive.CurrencyTypeCoins,
			ImageURL:     "https://example.com/diamond.png",
			AnimationURL: "https://example.com/diamond_fx.gif",
			Duration:     5 * time.Second,
			IsAvailable:  true,
		},
	}

	fmt.Println("\nGift Catalog:")
	for _, gift := range gifts {
		catalog.AddGift(gift)
		fmt.Printf("  %s (%s) - %d %s [%s]\n",
			gift.Name, gift.Rarity, gift.Price, gift.Currency, gift.Type)
	}

	// Give users coins
	cm.AddBalance("fan1", interactive.CurrencyTypeCoins, 1000, "Initial", nil)
	cm.AddBalance("fan2", interactive.CurrencyTypeCoins, 500, "Initial", nil)

	// Set gift callbacks
	gm.SetCallbacks(interactive.GiftCallbacks{
		OnGiftSent: func(giftSent *interactive.GiftSent, gift *interactive.Gift) {
			fmt.Printf("  → %s sent %dx %s to %s (Value: %d)\n",
				giftSent.FromUserID, giftSent.Amount, gift.Name, giftSent.ToUserID, giftSent.TotalValue)
		},
		OnComboUpdate: func(streamID, userID, giftID string, comboCount int) {
			fmt.Printf("  🔥 COMBO! %s sent %d gifts in combo!\n", userID, comboCount)
		},
	})

	// Send gifts
	fmt.Println("\nSending Gifts:")
	gm.SendGift("stream1", "rose", "fan1", "streamer1", 1, "Love your stream!")
	gm.SendGift("stream1", "rose", "fan1", "streamer1", 2, "")
	gm.SendGift("stream1", "heart", "fan2", "streamer1", 1, "Amazing content!")
	gm.SendGift("stream1", "rose", "fan1", "streamer1", 1, "") // Triggers combo

	// Get statistics
	stats := gm.GetStreamGiftStats("stream1")
	fmt.Println("\nStream Gift Statistics:")
	fmt.Printf("  Total Gifts: %d\n", stats.TotalGifts)
	fmt.Printf("  Total Value: %d coins\n", stats.TotalValue)

	// Get leaderboard
	leaderboard := gm.GetGiftLeaderboard("stream1", 3)
	fmt.Println("\nTop Gift Senders:")
	for _, entry := range leaderboard {
		fmt.Printf("  #%d %s - %d coins\n", entry.Rank, entry.UserID, entry.TotalValue)
	}
}

func runReactionExample() {
	fmt.Println("\n--- Reaction System Example ---")

	rm := interactive.NewReactionManager()
	rm.SetBurstThreshold(5, 2*time.Second)

	// Set callbacks
	rm.SetCallbacks(interactive.ReactionCallbacks{
		OnReaction: func(reaction *interactive.Reaction) {
			// Individual reactions logged silently
		},
		OnReactionBurst: func(streamID string, burst *interactive.ReactionBurst) {
			fmt.Printf("  🎆 BURST! %d x %s reactions (%.1f/sec)\n",
				burst.Count, burst.Type, burst.Intensity)
		},
		OnAggregateUpdate: func(aggregate *interactive.ReactionAggregate) {
			// Aggregate updated
		},
	})

	// Add custom reaction
	customReaction := &interactive.CustomReactionConfig{
		ID:           "custom1",
		Name:         "Fire",
		ImageURL:     "https://example.com/fire.png",
		AnimationURL: "https://example.com/fire.gif",
		Duration:     2 * time.Second,
	}
	rm.AddCustomReaction(customReaction)

	// Simulate reactions
	fmt.Println("\nSimulating Reactions:")
	reactionTypes := []interactive.ReactionType{
		interactive.ReactionTypeLike,
		interactive.ReactionTypeLove,
		interactive.ReactionTypeFire,
		interactive.ReactionTypeClap,
		interactive.ReactionTypeWow,
	}

	for i := 0; i < 20; i++ {
		reactionType := reactionTypes[i%len(reactionTypes)]
		userID := fmt.Sprintf("viewer%d", i%10)
		rm.AddReaction("stream1", userID, reactionType, "")
	}

	// Trigger a burst
	fmt.Println("\nTriggering Burst:")
	for i := 0; i < 8; i++ {
		rm.AddReaction("stream1", fmt.Sprintf("viewer%d", i), interactive.ReactionTypeFire, "")
	}

	// Get statistics
	stats := rm.GetReactionStats("stream1", 1*time.Minute)
	fmt.Println("\nReaction Statistics:")
	fmt.Printf("  Total Reactions: %d\n", stats.TotalReactions)
	fmt.Println("  By Type:")
	for reactionType, count := range stats.ByType {
		fmt.Printf("    %s: %d\n", reactionType, count)
	}

	// Get aggregate
	aggregate, _ := rm.GetStreamAggregate("stream1")
	fmt.Printf("  Recent Rate: %.2f reactions/sec\n", aggregate.RecentRate)

	// Top reactors
	topReactors := rm.GetTopReactors("stream1", 1*time.Minute, 3)
	fmt.Println("\nTop Reactors:")
	for _, reactor := range topReactors {
		fmt.Printf("  #%d %s - %d reactions\n", reactor.Rank, reactor.UserID, reactor.Count)
	}
}

func runMultiStreamExample() {
	fmt.Println("\n--- Multi-Stream (Co-Streaming) Example ---")

	msm := streaming.NewMultiStreamManager()

	// Set callbacks
	msm.SetCallbacks(streaming.MultiStreamCallbacks{
		OnSessionCreated: func(session *streaming.MultiStreamSession) {
			fmt.Printf("✓ Co-stream session created: %s\n", session.ID)
		},
		OnSessionStarted: func(session *streaming.MultiStreamSession) {
			fmt.Printf("✓ Co-stream session started\n")
		},
		OnSourceAdded: func(session *streaming.MultiStreamSession, source interface{}) {
			switch s := source.(type) {
			case *streaming.VideoSource:
				fmt.Printf("  → Video source added: %s (%s) - %dx%d\n",
					s.UserID, s.Type, s.Resolution.Width, s.Resolution.Height)
			case *streaming.AudioSource:
				fmt.Printf("  → Audio source added: %s - %dHz %dch\n",
					s.UserID, s.SampleRate, s.Channels)
			}
		},
		OnLayoutChanged: func(session *streaming.MultiStreamSession, layout *streaming.Layout) {
			fmt.Printf("  ✎ Layout changed to: %s\n", layout.Type)
		},
	})

	// Create session
	session, _ := msm.CreateSession("stream1", "host1", 4)
	msm.StartSession(session.ID)

	// Add video sources
	fmt.Println("\nAdding Sources:")
	resolution1080p := streaming.Resolution{Width: 1920, Height: 1080}
	resolution720p := streaming.Resolution{Width: 1280, Height: 720}

	// Host camera
	msm.AddVideoSource(session.ID, "host1", streaming.SourceTypeCamera,
		"rtmp://server/host1", resolution1080p)
	msm.AddAudioSource(session.ID, "host1", "rtmp://server/host1/audio", 48000, 2)

	// Guest screen share
	msm.AddVideoSource(session.ID, "guest1", streaming.SourceTypeScreen,
		"rtmp://server/guest1/screen", resolution1080p)
	msm.AddAudioSource(session.ID, "guest1", "rtmp://server/guest1/audio", 48000, 2)

	// Second guest camera
	msm.AddVideoSource(session.ID, "guest2", streaming.SourceTypeCamera,
		"rtmp://server/guest2", resolution720p)
	msm.AddAudioSource(session.ID, "guest2", "rtmp://server/guest2/audio", 44100, 2)

	// Get updated session
	session, _ = msm.GetSession(session.ID)
	fmt.Printf("\nSession Status:\n")
	fmt.Printf("  Video Sources: %d\n", len(session.VideoSources))
	fmt.Printf("  Audio Sources: %d\n", len(session.AudioSources))
	fmt.Printf("  Layout: %s\n", session.Layout.Type)

	// Demonstrate layout changes
	fmt.Println("\nChanging Layouts:")

	// PIP layout
	var mainSourceID string
	for id := range session.VideoSources {
		mainSourceID = id
		break
	}

	pipLayout := &streaming.Layout{
		Type:         streaming.LayoutTypePIP,
		MainSourceID: mainSourceID,
	}
	msm.SetLayout(session.ID, pipLayout)

	// Grid layout
	gridLayout := &streaming.Layout{
		Type:     streaming.LayoutTypeGrid,
		GridRows: 2,
		GridCols: 2,
	}
	msm.SetLayout(session.ID, gridLayout)

	// Audio mixing demonstration
	fmt.Println("\nAudio Mixing:")

	// Set volume levels
	for id, source := range session.AudioSources {
		if source.UserID == "host1" {
			msm.SetAudioVolume(session.ID, id, 1.0) // Host at full volume
			fmt.Printf("  %s: Volume 100%%\n", source.UserID)
		} else {
			msm.SetAudioVolume(session.ID, id, 0.7) // Guests at 70%
			fmt.Printf("  %s: Volume 70%%\n", source.UserID)
		}
	}

	// Mute one guest
	for id, source := range session.AudioSources {
		if source.UserID == "guest2" {
			msm.MuteAudioSource(session.ID, id, true)
			fmt.Printf("  %s: Muted\n", source.UserID)
		}
	}

	// Show active sessions
	activeSessions := msm.GetActiveSessions()
	fmt.Printf("\nActive Co-Stream Sessions: %d\n", len(activeSessions))

	// End session
	msm.EndSession(session.ID)
	fmt.Println("\n✓ Co-stream session ended")
}
