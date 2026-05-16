package repository

import (
	"testing"

	"fin-bot-miniapp/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLedgerTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&model.Ledger{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func createLedgerTestRepo(db *gorm.DB) LedgerRepository {
	return &ledgerRepo{db: db}
}

func TestLedgerRepository_Create(t *testing.T) {
	db := setupLedgerTestDB(t)
	repo := createLedgerTestRepo(db)

	ledger := &model.Ledger{
		UserID:    12345,
		Code:      "personal",
		Name:      "Personal",
		Emoji:     "📒",
		SortOrder: 1,
		Active:    true,
		IsDefault: true,
	}

	err := repo.Create(ledger)
	if err != nil {
		t.Fatalf("Failed to create ledger: %v", err)
	}

	if ledger.ID == 0 {
		t.Error("Expected ledger ID to be set after creation")
	}

	// Verify it's in the database
	var fetched model.Ledger
	err = db.First(&fetched, ledger.ID).Error
	if err != nil {
		t.Fatalf("Failed to fetch created ledger: %v", err)
	}

	if fetched.Code != "personal" {
		t.Errorf("Expected code 'personal', got '%s'", fetched.Code)
	}
}

func TestLedgerRepository_GetActiveByUserID(t *testing.T) {
	db := setupLedgerTestDB(t)
	repo := createLedgerTestRepo(db)

	// Create test ledgers
	ledgers := []*model.Ledger{
		{UserID: 12345, Code: "personal", Name: "Personal", Emoji: "📒", SortOrder: 1, Active: true, IsDefault: true},
		{UserID: 12345, Code: "business", Name: "Business", Emoji: "💼", SortOrder: 2, Active: true, IsDefault: false},
		{UserID: 12345, Code: "inactive", Name: "Inactive", Emoji: "❌", SortOrder: 3, Active: false, IsDefault: false},
		{UserID: 67890, Code: "other", Name: "Other User", Emoji: "👤", SortOrder: 1, Active: true, IsDefault: true},
	}

	for _, l := range ledgers {
		repo.Create(l)
	}

	// GORM's default:true tag overrides explicit false, so manually update inactive ledger
	db.Model(&model.Ledger{}).Where("code = ?", "inactive").Update("active", false)

	// Get active ledgers for user 12345
	results, err := repo.GetActiveByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get active ledgers: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 active ledgers, got %d", len(results))
		for i, r := range results {
			t.Logf("Result %d: UserID=%d, Code=%s, Active=%v", i, r.UserID, r.Code, r.Active)
		}
	}

	// Verify they're ordered by SortOrder
	if results[0].Code != "personal" {
		t.Errorf("Expected first ledger to be 'personal', got '%s'", results[0].Code)
	}
}

func TestLedgerRepository_SetDefault(t *testing.T) {
	db := setupLedgerTestDB(t)
	repo := createLedgerTestRepo(db)

	// Create test ledgers
	ledger1 := &model.Ledger{UserID: 12345, Code: "personal", Name: "Personal", Emoji: "📒", SortOrder: 1, Active: true, IsDefault: true}
	ledger2 := &model.Ledger{UserID: 12345, Code: "business", Name: "Business", Emoji: "💼", SortOrder: 2, Active: true, IsDefault: false}

	repo.Create(ledger1)
	repo.Create(ledger2)

	// Set ledger2 as default
	err := repo.SetDefault(12345, ledger2.ID)
	if err != nil {
		t.Fatalf("Failed to set default: %v", err)
	}

	// Verify ledger1 is no longer default
	var updated1 model.Ledger
	db.First(&updated1, ledger1.ID)
	if updated1.IsDefault {
		t.Error("Expected ledger1 to no longer be default")
	}

	// Verify ledger2 is now default
	var updated2 model.Ledger
	db.First(&updated2, ledger2.ID)
	if !updated2.IsDefault {
		t.Error("Expected ledger2 to be default")
	}
}

func TestLedgerRepository_GetDefaultByUserID(t *testing.T) {
	db := setupLedgerTestDB(t)
	repo := createLedgerTestRepo(db)

	// Create test ledgers
	ledger1 := &model.Ledger{UserID: 12345, Code: "personal", Name: "Personal", Emoji: "📒", SortOrder: 1, Active: true, IsDefault: false}
	ledger2 := &model.Ledger{UserID: 12345, Code: "business", Name: "Business", Emoji: "💼", SortOrder: 2, Active: true, IsDefault: true}

	repo.Create(ledger1)
	repo.Create(ledger2)

	// Get default ledger
	defaultLedger, err := repo.GetDefaultByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get default ledger: %v", err)
	}

	if defaultLedger.Code != "business" {
		t.Errorf("Expected default ledger to be 'business', got '%s'", defaultLedger.Code)
	}
}
