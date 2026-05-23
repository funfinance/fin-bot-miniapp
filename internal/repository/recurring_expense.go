package repository

import (
	"time"

	"fin-bot-miniapp/internal/database"
	"fin-bot-miniapp/internal/model"

	"gorm.io/gorm"
)

type RecurringExpenseRepository interface {
	Create(r *model.RecurringExpense) error
	GetDue(now time.Time) ([]*model.RecurringExpense, error)
	UpdateTrigger(id uint, lastTriggeredAt time.Time, nextTriggerAt time.Time) error
	GetActiveByUserID(userID int64) ([]*model.RecurringExpense, error)
	Update(r *model.RecurringExpense) error
	Deactivate(id uint, userID int64) error
}

type recurringExpenseRepo struct {
	db *gorm.DB
}

func NewRecurringExpenseRepository() RecurringExpenseRepository {
	return &recurringExpenseRepo{db: database.Get()}
}

func (r *recurringExpenseRepo) Create(rec *model.RecurringExpense) error {
	return r.db.Create(rec).Error
}

func (r *recurringExpenseRepo) GetDue(now time.Time) ([]*model.RecurringExpense, error) {
	var records []*model.RecurringExpense
	err := r.db.Where("active = ? AND next_trigger_at <= ?", true, now).Find(&records).Error
	return records, err
}

func (r *recurringExpenseRepo) UpdateTrigger(id uint, lastTriggeredAt time.Time, nextTriggerAt time.Time) error {
	return r.db.Model(&model.RecurringExpense{}).Where("id = ?", id).Updates(map[string]any{
		"last_triggered_at": lastTriggeredAt,
		"next_trigger_at":   nextTriggerAt,
	}).Error
}

func (r *recurringExpenseRepo) GetActiveByUserID(userID int64) ([]*model.RecurringExpense, error) {
	var records []*model.RecurringExpense
	err := r.db.Where("user_id = ? AND active = ?", userID, true).Find(&records).Error
	return records, err
}

func (r *recurringExpenseRepo) Update(rec *model.RecurringExpense) error {
	return r.db.Model(rec).Where("id = ? AND user_id = ?", rec.ID, rec.UserID).Updates(map[string]any{
		"ledger_id":       rec.LedgerID,
		"type":            rec.Type,
		"amount":          rec.Amount,
		"currency":        rec.Currency,
		"category":        rec.Category,
		"description":     rec.Description,
		"frequency":       rec.Frequency,
		"days":            rec.Days,
		"next_trigger_at": rec.NextTriggerAt,
	}).Error
}

func (r *recurringExpenseRepo) Deactivate(id uint, userID int64) error {
	return r.db.Model(&model.RecurringExpense{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("active", false).Error
}
