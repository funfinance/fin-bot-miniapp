package service

import (
	"testing"

	"fin-bot-miniapp/internal/model"
	"fin-bot-miniapp/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupLedgerServiceTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	err = db.AutoMigrate(&model.Ledger{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func createTestLedgerRepo(db *gorm.DB) repository.LedgerRepository {
	return &testLedgerRepoWrapper{db: db}
}

type testLedgerRepoWrapper struct {
	db *gorm.DB
}

func (r *testLedgerRepoWrapper) GetByUserID(userID int64) ([]*model.Ledger, error) {
	var ledgers []*model.Ledger
	err := r.db.Where("user_id = ?", userID).Order("sort_order ASC").Find(&ledgers).Error
	return ledgers, err
}

func (r *testLedgerRepoWrapper) GetActiveByUserID(userID int64) ([]*model.Ledger, error) {
	var ledgers []*model.Ledger
	err := r.db.Where("user_id = ? AND active = ?", userID, true).Order("sort_order ASC").Find(&ledgers).Error
	return ledgers, err
}

func (r *testLedgerRepoWrapper) GetByUserIDAndCode(userID int64, code string) (*model.Ledger, error) {
	var ledger model.Ledger
	err := r.db.Where("user_id = ? AND code = ?", userID, code).First(&ledger).Error
	return &ledger, err
}

func (r *testLedgerRepoWrapper) GetDefaultByUserID(userID int64) (*model.Ledger, error) {
	var ledger model.Ledger
	err := r.db.Where("user_id = ? AND is_default = ?", userID, true).First(&ledger).Error
	return &ledger, err
}

func (r *testLedgerRepoWrapper) GetByID(id uint) (*model.Ledger, error) {
	var ledger model.Ledger
	err := r.db.First(&ledger, id).Error
	return &ledger, err
}

func (r *testLedgerRepoWrapper) Create(ledger *model.Ledger) error {
	return r.db.Create(ledger).Error
}

func (r *testLedgerRepoWrapper) Upsert(ledger *model.Ledger) error {
	return r.db.Save(ledger).Error
}

func (r *testLedgerRepoWrapper) SetDefault(userID int64, ledgerID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Ledger{}).Where("user_id = ?", userID).Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&model.Ledger{}).Where("id = ? AND user_id = ?", ledgerID, userID).Update("is_default", true).Error
	})
}

func TestLedgerService_InitializeDefaultLedger(t *testing.T) {
	db := setupLedgerServiceTestDB(t)
	repo := createTestLedgerRepo(db)
	service := NewLedgerService(repo)

	ledgers, err := service.GetActiveByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get ledgers: %v", err)
	}

	if len(ledgers) != 1 {
		t.Errorf("Expected 1 default ledger, got %d", len(ledgers))
	}

	if ledgers[0].Code != "default" {
		t.Errorf("Expected default ledger code 'default', got '%s'", ledgers[0].Code)
	}

	if !ledgers[0].IsDefault {
		t.Error("Expected default ledger to have IsDefault=true")
	}
}

func TestLedgerService_AddLedger(t *testing.T) {
	db := setupLedgerServiceTestDB(t)
	repo := createTestLedgerRepo(db)
	service := NewLedgerService(repo)

	service.GetActiveByUserID(12345)

	err := service.AddLedger(12345, "personal", "Personal", "📒", 2)
	if err != nil {
		t.Fatalf("Failed to add ledger: %v", err)
	}

	ledgers, err := service.GetActiveByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get ledgers: %v", err)
	}

	if len(ledgers) != 2 {
		t.Errorf("Expected 2 ledgers, got %d", len(ledgers))
	}

	err = service.AddLedger(12345, "personal", "Personal 2", "📘", 3)
	if err != ErrLedgerCodeExists {
		t.Errorf("Expected ErrLedgerCodeExists, got %v", err)
	}
}

func TestLedgerService_GetDefaultByUserID(t *testing.T) {
	db := setupLedgerServiceTestDB(t)
	repo := createTestLedgerRepo(db)
	service := NewLedgerService(repo)

	defaultLedger, err := service.GetDefaultByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get default ledger: %v", err)
	}

	if !defaultLedger.IsDefault {
		t.Error("Expected ledger to be default")
	}

	if defaultLedger.Code != "default" {
		t.Errorf("Expected code 'default', got '%s'", defaultLedger.Code)
	}
}

func TestLedgerService_SetDefault(t *testing.T) {
	db := setupLedgerServiceTestDB(t)
	repo := createTestLedgerRepo(db)
	service := NewLedgerService(repo)

	service.GetActiveByUserID(12345)
	service.AddLedger(12345, "business", "Business", "💼", 2)

	ledgers, _ := service.GetActiveByUserID(12345)

	var businessLedger *model.Ledger
	for _, l := range ledgers {
		if l.Code == "business" {
			businessLedger = l
			break
		}
	}

	err := service.SetDefault(12345, businessLedger.ID)
	if err != nil {
		t.Fatalf("Failed to set default: %v", err)
	}

	defaultLedger, err := service.GetDefaultByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get default ledger: %v", err)
	}

	if defaultLedger.Code != "business" {
		t.Errorf("Expected default to be 'business', got '%s'", defaultLedger.Code)
	}
}

func TestLedgerService_GetDefaultByUserID_FallbackToFirst(t *testing.T) {
	db := setupLedgerServiceTestDB(t)
	repo := createTestLedgerRepo(db)
	service := NewLedgerService(repo)

	db.Create(&model.Ledger{UserID: 99999, Code: "a", Name: "A", Emoji: "📒", SortOrder: 1, Active: true, IsDefault: false})
	db.Create(&model.Ledger{UserID: 99999, Code: "b", Name: "B", Emoji: "📘", SortOrder: 2, Active: true, IsDefault: false})

	ledger, err := service.GetDefaultByUserID(99999)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if ledger.Code != "a" {
		t.Errorf("Expected first ledger 'a', got '%s'", ledger.Code)
	}
}

func TestLedgerService_Cache(t *testing.T) {
	db := setupLedgerServiceTestDB(t)
	repo := createTestLedgerRepo(db)
	service := NewLedgerService(repo)

	ledgers1, err := service.GetActiveByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get ledgers: %v", err)
	}

	ledgers2, err := service.GetActiveByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get ledgers from cache: %v", err)
	}

	if len(ledgers1) != len(ledgers2) {
		t.Error("Cache returned different results")
	}

	service.AddLedger(12345, "personal", "Personal", "📒", 2)

	ledgers3, err := service.GetActiveByUserID(12345)
	if err != nil {
		t.Fatalf("Failed to get ledgers after cache invalidation: %v", err)
	}

	if len(ledgers3) != len(ledgers1)+1 {
		t.Error("Cache was not properly invalidated after adding ledger")
	}
}
