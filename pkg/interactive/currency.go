package interactive

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// CurrencyType represents the type of currency
type CurrencyType string

const (
	// CurrencyTypeCoins represents virtual coins
	CurrencyTypeCoins CurrencyType = "coins"
	// CurrencyTypeDiamonds represents premium currency (diamonds)
	CurrencyTypeDiamonds CurrencyType = "diamonds"
	// CurrencyTypePoints represents loyalty points
	CurrencyTypePoints CurrencyType = "points"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	// TransactionTypeEarn represents earning currency
	TransactionTypeEarn TransactionType = "earn"
	// TransactionTypeSpend represents spending currency
	TransactionTypeSpend TransactionType = "spend"
	// TransactionTypeTransfer represents transferring currency to another user
	TransactionTypeTransfer TransactionType = "transfer"
	// TransactionTypePurchase represents purchasing currency with real money
	TransactionTypePurchase TransactionType = "purchase"
	// TransactionTypeRefund represents refunding currency
	TransactionTypeRefund TransactionType = "refund"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	// TransactionStatusPending indicates the transaction is pending
	TransactionStatusPending TransactionStatus = "pending"
	// TransactionStatusCompleted indicates the transaction is completed
	TransactionStatusCompleted TransactionStatus = "completed"
	// TransactionStatusFailed indicates the transaction failed
	TransactionStatusFailed TransactionStatus = "failed"
	// TransactionStatusRefunded indicates the transaction was refunded
	TransactionStatusRefunded TransactionStatus = "refunded"
)

// Balance represents a user's currency balance
type Balance struct {
	UserID    string                 `json:"user_id"`
	Balances  map[CurrencyType]int64 `json:"balances"`
	UpdatedAt time.Time              `json:"updated_at"`
	mu        sync.RWMutex           `json:"-"`
}

// Transaction represents a currency transaction
type Transaction struct {
	ID            string            `json:"id"`
	UserID        string            `json:"user_id"`
	Type          TransactionType   `json:"type"`
	CurrencyType  CurrencyType      `json:"currency_type"`
	Amount        int64             `json:"amount"`
	Balance       int64             `json:"balance_after"`
	Status        TransactionStatus `json:"status"`
	Description   string            `json:"description"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	RelatedUserID string            `json:"related_user_id,omitempty"` // For transfers
	RelatedItemID string            `json:"related_item_id,omitempty"` // For purchases (e.g., gift ID)
}

// CurrencyManager manages virtual currency for users
type CurrencyManager struct {
	balances     map[string]*Balance     // userID -> Balance
	transactions map[string]*Transaction // transactionID -> Transaction
	userTxns     map[string][]string     // userID -> []transactionID
	mu           sync.RWMutex
	callbacks    CurrencyCallbacks
}

// CurrencyCallbacks defines callback functions for currency events
type CurrencyCallbacks struct {
	OnBalanceChanged     func(userID string, currencyType CurrencyType, oldBalance, newBalance int64)
	OnTransactionCreated func(transaction *Transaction)
	OnTransactionFailed  func(transaction *Transaction, err error)
}

// NewCurrencyManager creates a new currency manager
func NewCurrencyManager() *CurrencyManager {
	return &CurrencyManager{
		balances:     make(map[string]*Balance),
		transactions: make(map[string]*Transaction),
		userTxns:     make(map[string][]string),
	}
}

// SetCallbacks sets the callback functions for currency events
func (cm *CurrencyManager) SetCallbacks(callbacks CurrencyCallbacks) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.callbacks = callbacks
}

// GetBalance returns the user's balance for a specific currency type
func (cm *CurrencyManager) GetBalance(userID string, currencyType CurrencyType) (int64, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	balance, exists := cm.balances[userID]
	if !exists {
		return 0, nil // New user has 0 balance
	}

	balance.mu.RLock()
	defer balance.mu.RUnlock()

	return balance.Balances[currencyType], nil
}

// GetAllBalances returns all balances for a user
func (cm *CurrencyManager) GetAllBalances(userID string) (map[CurrencyType]int64, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	balance, exists := cm.balances[userID]
	if !exists {
		// Return zero balances for new user
		return map[CurrencyType]int64{
			CurrencyTypeCoins:    0,
			CurrencyTypeDiamonds: 0,
			CurrencyTypePoints:   0,
		}, nil
	}

	balance.mu.RLock()
	defer balance.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[CurrencyType]int64)
	for k, v := range balance.Balances {
		result[k] = v
	}

	return result, nil
}

// AddBalance adds currency to a user's balance
func (cm *CurrencyManager) AddBalance(userID string, currencyType CurrencyType, amount int64, description string, metadata map[string]string) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	return cm.updateBalance(userID, currencyType, amount, TransactionTypeEarn, description, metadata, "")
}

// DeductBalance deducts currency from a user's balance
func (cm *CurrencyManager) DeductBalance(userID string, currencyType CurrencyType, amount int64, description string, metadata map[string]string) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	return cm.updateBalance(userID, currencyType, -amount, TransactionTypeSpend, description, metadata, "")
}

// Transfer transfers currency from one user to another
func (cm *CurrencyManager) Transfer(fromUserID, toUserID string, currencyType CurrencyType, amount int64, description string) (*Transaction, *Transaction, error) {
	if amount <= 0 {
		return nil, nil, errors.New("amount must be positive")
	}

	if fromUserID == toUserID {
		return nil, nil, errors.New("cannot transfer to self")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check sender's balance
	fromBalance := cm.getOrCreateBalance(fromUserID)
	fromBalance.mu.Lock()
	defer fromBalance.mu.Unlock()

	if fromBalance.Balances[currencyType] < amount {
		return nil, nil, fmt.Errorf("insufficient balance: have %d, need %d", fromBalance.Balances[currencyType], amount)
	}

	// Deduct from sender
	oldFromBalance := fromBalance.Balances[currencyType]
	fromBalance.Balances[currencyType] -= amount
	fromBalance.UpdatedAt = time.Now()

	fromTxn := &Transaction{
		ID:            generateTransactionID(),
		UserID:        fromUserID,
		Type:          TransactionTypeTransfer,
		CurrencyType:  currencyType,
		Amount:        -amount,
		Balance:       fromBalance.Balances[currencyType],
		Status:        TransactionStatusCompleted,
		Description:   description,
		Metadata:      map[string]string{"transfer_to": toUserID},
		RelatedUserID: toUserID,
		CreatedAt:     time.Now(),
	}
	now := time.Now()
	fromTxn.CompletedAt = &now

	cm.transactions[fromTxn.ID] = fromTxn
	cm.userTxns[fromUserID] = append(cm.userTxns[fromUserID], fromTxn.ID)

	// Add to receiver
	toBalance := cm.getOrCreateBalance(toUserID)
	toBalance.mu.Lock()
	defer toBalance.mu.Unlock()

	oldToBalance := toBalance.Balances[currencyType]
	toBalance.Balances[currencyType] += amount
	toBalance.UpdatedAt = time.Now()

	toTxn := &Transaction{
		ID:            generateTransactionID(),
		UserID:        toUserID,
		Type:          TransactionTypeTransfer,
		CurrencyType:  currencyType,
		Amount:        amount,
		Balance:       toBalance.Balances[currencyType],
		Status:        TransactionStatusCompleted,
		Description:   description,
		Metadata:      map[string]string{"transfer_from": fromUserID},
		RelatedUserID: fromUserID,
		CreatedAt:     time.Now(),
		CompletedAt:   &now,
	}

	cm.transactions[toTxn.ID] = toTxn
	cm.userTxns[toUserID] = append(cm.userTxns[toUserID], toTxn.ID)

	// Trigger callbacks
	if cm.callbacks.OnBalanceChanged != nil {
		cm.callbacks.OnBalanceChanged(fromUserID, currencyType, oldFromBalance, fromBalance.Balances[currencyType])
		cm.callbacks.OnBalanceChanged(toUserID, currencyType, oldToBalance, toBalance.Balances[currencyType])
	}
	if cm.callbacks.OnTransactionCreated != nil {
		cm.callbacks.OnTransactionCreated(fromTxn)
		cm.callbacks.OnTransactionCreated(toTxn)
	}

	return fromTxn, toTxn, nil
}

// updateBalance is an internal method to update balance
func (cm *CurrencyManager) updateBalance(userID string, currencyType CurrencyType, amount int64, txnType TransactionType, description string, metadata map[string]string, relatedItemID string) (*Transaction, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	balance := cm.getOrCreateBalance(userID)
	balance.mu.Lock()
	defer balance.mu.Unlock()

	oldBalance := balance.Balances[currencyType]
	newBalance := oldBalance + amount

	// Check for negative balance
	if newBalance < 0 {
		return nil, fmt.Errorf("insufficient balance: have %d, need %d", oldBalance, -amount)
	}

	balance.Balances[currencyType] = newBalance
	balance.UpdatedAt = time.Now()

	// Create transaction record
	txn := &Transaction{
		ID:            generateTransactionID(),
		UserID:        userID,
		Type:          txnType,
		CurrencyType:  currencyType,
		Amount:        amount,
		Balance:       newBalance,
		Status:        TransactionStatusCompleted,
		Description:   description,
		Metadata:      metadata,
		RelatedItemID: relatedItemID,
		CreatedAt:     time.Now(),
	}
	now := time.Now()
	txn.CompletedAt = &now

	cm.transactions[txn.ID] = txn
	cm.userTxns[userID] = append(cm.userTxns[userID], txn.ID)

	// Trigger callbacks
	if cm.callbacks.OnBalanceChanged != nil {
		cm.callbacks.OnBalanceChanged(userID, currencyType, oldBalance, newBalance)
	}
	if cm.callbacks.OnTransactionCreated != nil {
		cm.callbacks.OnTransactionCreated(txn)
	}

	return txn, nil
}

// getOrCreateBalance gets or creates a balance for a user (must be called with lock held)
func (cm *CurrencyManager) getOrCreateBalance(userID string) *Balance {
	balance, exists := cm.balances[userID]
	if !exists {
		balance = &Balance{
			UserID: userID,
			Balances: map[CurrencyType]int64{
				CurrencyTypeCoins:    0,
				CurrencyTypeDiamonds: 0,
				CurrencyTypePoints:   0,
			},
			UpdatedAt: time.Now(),
		}
		cm.balances[userID] = balance
	}
	return balance
}

// GetTransaction returns a transaction by ID
func (cm *CurrencyManager) GetTransaction(transactionID string) (*Transaction, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	txn, exists := cm.transactions[transactionID]
	if !exists {
		return nil, fmt.Errorf("transaction not found: %s", transactionID)
	}

	return txn, nil
}

// GetUserTransactions returns all transactions for a user
func (cm *CurrencyManager) GetUserTransactions(userID string, limit int) ([]*Transaction, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	txnIDs := cm.userTxns[userID]
	if len(txnIDs) == 0 {
		return []*Transaction{}, nil
	}

	// Get transactions in reverse order (newest first)
	start := len(txnIDs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*Transaction, 0, limit)
	for i := len(txnIDs) - 1; i >= start; i-- {
		txn := cm.transactions[txnIDs[i]]
		result = append(result, txn)
	}

	return result, nil
}

// GetUserTransactionsByCurrency returns transactions for a specific currency type
func (cm *CurrencyManager) GetUserTransactionsByCurrency(userID string, currencyType CurrencyType, limit int) ([]*Transaction, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	txnIDs := cm.userTxns[userID]
	if len(txnIDs) == 0 {
		return []*Transaction{}, nil
	}

	result := make([]*Transaction, 0)
	for i := len(txnIDs) - 1; i >= 0 && len(result) < limit; i-- {
		txn := cm.transactions[txnIDs[i]]
		if txn.CurrencyType == currencyType {
			result = append(result, txn)
		}
	}

	return result, nil
}

// Purchase creates a purchase transaction (for buying currency with real money)
func (cm *CurrencyManager) Purchase(userID string, currencyType CurrencyType, amount int64, realMoney float64, paymentMethod string) (*Transaction, error) {
	if amount <= 0 {
		return nil, errors.New("amount must be positive")
	}

	metadata := map[string]string{
		"real_money":     fmt.Sprintf("%.2f", realMoney),
		"payment_method": paymentMethod,
	}

	return cm.updateBalance(userID, currencyType, amount, TransactionTypePurchase, fmt.Sprintf("Purchased %d %s", amount, currencyType), metadata, "")
}

// Refund creates a refund transaction
func (cm *CurrencyManager) Refund(transactionID string, reason string) (*Transaction, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	originalTxn, exists := cm.transactions[transactionID]
	if !exists {
		return nil, fmt.Errorf("transaction not found: %s", transactionID)
	}

	if originalTxn.Status == TransactionStatusRefunded {
		return nil, errors.New("transaction already refunded")
	}

	// Create refund transaction (reverse the original amount)
	balance := cm.getOrCreateBalance(originalTxn.UserID)
	balance.mu.Lock()
	defer balance.mu.Unlock()

	oldBalance := balance.Balances[originalTxn.CurrencyType]
	refundAmount := -originalTxn.Amount
	newBalance := oldBalance + refundAmount

	if newBalance < 0 {
		return nil, errors.New("cannot refund: would result in negative balance")
	}

	balance.Balances[originalTxn.CurrencyType] = newBalance
	balance.UpdatedAt = time.Now()

	refundTxn := &Transaction{
		ID:            generateTransactionID(),
		UserID:        originalTxn.UserID,
		Type:          TransactionTypeRefund,
		CurrencyType:  originalTxn.CurrencyType,
		Amount:        refundAmount,
		Balance:       newBalance,
		Status:        TransactionStatusCompleted,
		Description:   fmt.Sprintf("Refund for transaction %s: %s", transactionID, reason),
		Metadata:      map[string]string{"original_transaction": transactionID, "reason": reason},
		RelatedItemID: transactionID,
		CreatedAt:     time.Now(),
	}
	now := time.Now()
	refundTxn.CompletedAt = &now

	// Update original transaction status
	originalTxn.Status = TransactionStatusRefunded

	cm.transactions[refundTxn.ID] = refundTxn
	cm.userTxns[originalTxn.UserID] = append(cm.userTxns[originalTxn.UserID], refundTxn.ID)

	// Trigger callbacks
	if cm.callbacks.OnBalanceChanged != nil {
		cm.callbacks.OnBalanceChanged(originalTxn.UserID, originalTxn.CurrencyType, oldBalance, newBalance)
	}
	if cm.callbacks.OnTransactionCreated != nil {
		cm.callbacks.OnTransactionCreated(refundTxn)
	}

	return refundTxn, nil
}

// GetBalanceHistory returns the balance history for a currency type
func (cm *CurrencyManager) GetBalanceHistory(userID string, currencyType CurrencyType) ([]*BalanceSnapshot, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	txnIDs := cm.userTxns[userID]
	snapshots := make([]*BalanceSnapshot, 0, len(txnIDs))

	for _, txnID := range txnIDs {
		txn := cm.transactions[txnID]
		if txn.CurrencyType == currencyType {
			snapshots = append(snapshots, &BalanceSnapshot{
				Timestamp: txn.CreatedAt,
				Balance:   txn.Balance,
				Change:    txn.Amount,
				Type:      txn.Type,
			})
		}
	}

	return snapshots, nil
}

// BalanceSnapshot represents a point-in-time balance snapshot
type BalanceSnapshot struct {
	Timestamp time.Time       `json:"timestamp"`
	Balance   int64           `json:"balance"`
	Change    int64           `json:"change"`
	Type      TransactionType `json:"type"`
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID() string {
	return fmt.Sprintf("txn_%d", time.Now().UnixNano())
}
