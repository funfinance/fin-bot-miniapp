package repository

import (
	"fin-bot-miniapp/internal/database"
	"fin-bot-miniapp/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LedgerRepository defines ledger data access interface
type LedgerRepository interface {
	GetByUserID(userID int64) ([]*model.Ledger, error)
	GetActiveByUserID(userID int64) ([]*model.Ledger, error)
	GetByUserIDAndCode(userID int64, code string) (*model.Ledger, error)
	GetDefaultByUserID(userID int64) (*model.Ledger, error)
	GetByID(id uint) (*model.Ledger, error)
	Create(ledger *model.Ledger) error
	Upsert(ledger *model.Ledger) error
	SetDefault(userID int64, ledgerID uint) error
}

type ledgerRepo struct {
	db *gorm.DB
}

// NewLedgerRepository creates a new ledger repository instance
func NewLedgerRepository() LedgerRepository {
	return &ledgerRepo{
		db: database.Get(),
	}
}

func (r *ledgerRepo) GetByUserID(userID int64) ([]*model.Ledger, error) {
	var ledgers []*model.Ledger
	err := r.db.Where("user_id = ?", userID).
		Order("sort_order ASC, name ASC").
		Find(&ledgers).Error
	return ledgers, err
}

func (r *ledgerRepo) GetActiveByUserID(userID int64) ([]*model.Ledger, error) {
	var ledgers []*model.Ledger
	err := r.db.Where("user_id = ? AND active = ?", userID, true).
		Order("sort_order ASC, name ASC").
		Find(&ledgers).Error
	return ledgers, err
}

func (r *ledgerRepo) GetByUserIDAndCode(userID int64, code string) (*model.Ledger, error) {
	var ledger model.Ledger
	err := r.db.Where("user_id = ? AND code = ?", userID, code).First(&ledger).Error
	if err != nil {
		return nil, err
	}
	return &ledger, nil
}

func (r *ledgerRepo) GetDefaultByUserID(userID int64) (*model.Ledger, error) {
	var ledger model.Ledger
	err := r.db.Where("user_id = ? AND is_default = ?", userID, true).First(&ledger).Error
	if err != nil {
		return nil, err
	}
	return &ledger, nil
}

func (r *ledgerRepo) GetByID(id uint) (*model.Ledger, error) {
	var ledger model.Ledger
	err := r.db.Where("id = ?", id).First(&ledger).Error
	if err != nil {
		return nil, err
	}
	return &ledger, nil
}

func (r *ledgerRepo) Create(ledger *model.Ledger) error {
	return r.db.Create(ledger).Error
}

func (r *ledgerRepo) Upsert(ledger *model.Ledger) error {
	// Use GORM's Clauses for atomic upsert
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "code"}},
		UpdateAll: true,
	}).Create(ledger).Error
}

func (r *ledgerRepo) SetDefault(userID int64, ledgerID uint) error {
	// Use transaction to ensure atomicity
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Unset all defaults for this user
		if err := tx.Model(&model.Ledger{}).
			Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}

		// Set the new default
		if err := tx.Model(&model.Ledger{}).
			Where("id = ? AND user_id = ?", ledgerID, userID).
			Update("is_default", true).Error; err != nil {
			return err
		}

		return nil
	})
}
