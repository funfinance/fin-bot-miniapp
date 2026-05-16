package repository

import (
	"fin-bot-miniapp/internal/database"
	"fin-bot-miniapp/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RateRepository defines exchange rate data access interface
type RateRepository interface {
	Upsert(rate *model.ExchangeRate) error
	GetByCurrency(currency string) (*model.ExchangeRate, error)
	GetAll() ([]*model.ExchangeRate, error)
}

type rateRepo struct {
	db *gorm.DB
}

// NewRateRepository creates a new rate repository instance
func NewRateRepository() RateRepository {
	return &rateRepo{
		db: database.Get(),
	}
}

func (r *rateRepo) Upsert(rate *model.ExchangeRate) error {
	// Use GORM's Clauses for atomic upsert (INSERT ... ON CONFLICT UPDATE)
	// This is thread-safe and prevents race conditions
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "currency"}},                              // conflict column
		DoUpdates: clause.AssignmentColumns([]string{"rate_to_base", "updated_at"}), // columns to update
	}).Create(rate).Error
}

func (r *rateRepo) GetByCurrency(currency string) (*model.ExchangeRate, error) {
	var rate model.ExchangeRate
	err := r.db.Where("currency = ?", currency).First(&rate).Error
	if err != nil {
		return nil, err
	}
	return &rate, nil
}

func (r *rateRepo) GetAll() ([]*model.ExchangeRate, error) {
	var rates []*model.ExchangeRate
	err := r.db.Find(&rates).Error
	return rates, err
}
