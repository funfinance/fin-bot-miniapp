package model

import (
	"time"
)

// Expense represents an expense or income record
type Expense struct {
	ID           uint      `gorm:"primaryKey"`
	UserID       int64     `gorm:"index;not null"`
	Username     string    `gorm:"size:100"`
	LedgerID     uint      `gorm:"index;not null;default:0"`                 // Which ledger this expense belongs to
	Type         string    `gorm:"size:20;not null;default:'expense';index"` // "expense" or "income"
	Amount       float64   `gorm:"not null"`
	Currency     string    `gorm:"size:10;default:'JPY'"` // For income, defaults to JPY
	AmountInBase float64   `gorm:"not null"`              // Converted to base currency for unified statistics
	Category     string    `gorm:"size:50;index"`         // Optional for income
	Description  string    `gorm:"size:200"`
	ExpenseDate  time.Time `gorm:"index"` // Optional for income, defaults to current time
	CreatedAt    time.Time // Record creation time
	UpdatedAt    time.Time
}

// TableName overrides the table name
func (Expense) TableName() string {
	return "expenses"
}

// ExchangeRate represents currency exchange rate
type ExchangeRate struct {
	ID         uint    `gorm:"primaryKey"`
	Currency   string  `gorm:"size:10;uniqueIndex;not null"`
	RateToBase float64 `gorm:"not null"` // Rate relative to base currency (1 base = X this_currency)
	UpdatedAt  time.Time
}

// TableName overrides the table name
func (ExchangeRate) TableName() string {
	return "exchange_rates"
}

// Category represents an expense category
type Category struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    int64  `gorm:"uniqueIndex:idx_user_code;not null"`         // Owner of this category
	Code      string `gorm:"size:50;not null;uniqueIndex:idx_user_code"` // Unique code within user's categories
	Name      string `gorm:"size:100;not null"`                          // Display name
	Emoji     string `gorm:"size:10;not null"`                           // Emoji icon
	SortOrder int    `gorm:"not null;default:0"`                         // Display order
	Active    bool   `gorm:"not null;default:true"`                      // Is active
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName overrides the table name
func (Category) TableName() string {
	return "categories"
}

// Ledger represents a ledger/account book for organizing expenses
type Ledger struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    int64  `gorm:"index;not null"`         // Owner of this ledger
	Code      string `gorm:"size:50;not null"`       // Unique code within user's ledgers
	Name      string `gorm:"size:100;not null"`      // Display name
	Emoji     string `gorm:"size:10;not null"`       // Emoji icon
	SortOrder int    `gorm:"not null;default:0"`     // Display order
	Active    bool   `gorm:"not null;default:true"`  // Is active
	IsDefault bool   `gorm:"not null;default:false"` // Is default ledger
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName overrides the table name
func (Ledger) TableName() string {
	return "ledgers"
}

// RecurringExpense defines a recurring expense rule that auto-creates expenses on a schedule.
type RecurringExpense struct {
	ID              uint       `gorm:"primaryKey"`
	UserID          int64      `gorm:"index;not null"`
	Username        string     `gorm:"size:100"`
	LedgerID        uint       `gorm:"not null;default:0"`
	Type            string     `gorm:"size:20;not null;default:'expense'"` // "expense" or "income"
	Amount          float64    `gorm:"not null"`
	Currency        string     `gorm:"size:10;not null"`
	Category        string     `gorm:"size:50"`
	Description     string     `gorm:"size:200"`
	Frequency       string     `gorm:"size:20;not null"` // "weekly" or "monthly"
	Days            string     `gorm:"size:100;not null"` // comma-separated: weekday 0-6 or month day 1-31
	NextTriggerAt   time.Time  `gorm:"index;not null"`
	LastTriggeredAt *time.Time // nil until first trigger
	Active          bool       `gorm:"not null;default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (RecurringExpense) TableName() string {
	return "recurring_expenses"
}
