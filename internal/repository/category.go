package repository

import (
	"fin-bot-miniapp/internal/database"
	"fin-bot-miniapp/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CategoryRepository defines category data access interface
type CategoryRepository interface {
	GetByUserID(userID int64) ([]*model.Category, error)
	GetActiveByUserID(userID int64) ([]*model.Category, error)
	GetByUserIDAndCode(userID int64, code string) (*model.Category, error)
	Upsert(category *model.Category) error
}

type categoryRepo struct {
	db *gorm.DB
}

// NewCategoryRepository creates a new category repository instance
func NewCategoryRepository() CategoryRepository {
	return &categoryRepo{
		db: database.Get(),
	}
}

func (r *categoryRepo) GetByUserID(userID int64) ([]*model.Category, error) {
	var categories []*model.Category
	err := r.db.Where("user_id = ?", userID).
		Order("sort_order ASC, name ASC").
		Find(&categories).Error
	return categories, err
}

func (r *categoryRepo) GetActiveByUserID(userID int64) ([]*model.Category, error) {
	var categories []*model.Category
	err := r.db.Where("user_id = ? AND active = ?", userID, true).
		Order("sort_order ASC, name ASC").
		Find(&categories).Error
	return categories, err
}

func (r *categoryRepo) GetByUserIDAndCode(userID int64, code string) (*model.Category, error) {
	var category model.Category
	err := r.db.Where("user_id = ? AND code = ?", userID, code).First(&category).Error
	return &category, err
}

func (r *categoryRepo) Upsert(category *model.Category) error {
	// Use GORM's Clauses for atomic upsert (INSERT ... ON CONFLICT UPDATE)
	// This is thread-safe and prevents race conditions
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "code"}}, // conflict columns (composite key)
		UpdateAll: true,                                               // update all fields on conflict
	}).Create(category).Error
}
