package repository

import (
	"fmt"
	"time"

	"fin-bot-miniapp/internal/database"
	"fin-bot-miniapp/internal/model"

	"gorm.io/gorm"
)

// ExpenseRepository defines expense data access interface
type ExpenseRepository interface {
	Create(expense *model.Expense) error
	GetByUserID(userID int64, limit int) ([]*model.Expense, error)
	GetByUserIDAndDateRange(userID int64, start, end time.Time) ([]*model.Expense, error)
	GetByUserIDLedgerIDAndDateRange(userID int64, ledgerID uint, start, end time.Time) ([]*model.Expense, error)
	GetMostRecent(userID int64) (*model.Expense, error)
	Delete(id uint, userID int64) error
	GetYears(userID int64) ([]int, error)
	GetMonths(userID int64, year int) ([]int, error)
	GetByYearMonth(userID int64, year, month int) ([]*model.Expense, error)
	Update(expense *model.Expense) error
}

type expenseRepo struct {
	db *gorm.DB
}

// NewExpenseRepository creates a new expense repository instance
func NewExpenseRepository() ExpenseRepository {
	return &expenseRepo{
		db: database.Get(),
	}
}

func (r *expenseRepo) Create(expense *model.Expense) error {
	return r.db.Create(expense).Error
}

func (r *expenseRepo) GetByUserID(userID int64, limit int) ([]*model.Expense, error) {
	var expenses []*model.Expense
	err := r.db.Where("user_id = ?", userID).
		Order("expense_date DESC, created_at DESC").
		Limit(limit).
		Find(&expenses).Error

	return expenses, err
}

func (r *expenseRepo) GetByUserIDAndDateRange(userID int64, start, end time.Time) ([]*model.Expense, error) {
	var expenses []*model.Expense
	err := r.db.Where("user_id = ? AND expense_date BETWEEN ? AND ?", userID, start, end).
		Order("expense_date DESC, created_at DESC").
		Find(&expenses).Error

	return expenses, err
}

func (r *expenseRepo) GetByUserIDLedgerIDAndDateRange(userID int64, ledgerID uint, start, end time.Time) ([]*model.Expense, error) {
	var expenses []*model.Expense
	err := r.db.Where("user_id = ? AND ledger_id = ? AND expense_date BETWEEN ? AND ?", userID, ledgerID, start, end).
		Order("expense_date DESC, created_at DESC").
		Find(&expenses).Error

	return expenses, err
}

// GetMostRecent returns the most recently created expense or income for the user.
func (r *expenseRepo) GetMostRecent(userID int64) (*model.Expense, error) {
	var expense model.Expense
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&expense).Error
	if err != nil {
		return nil, err
	}
	return &expense, nil
}

func (r *expenseRepo) Delete(id uint, userID int64) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).
		Delete(&model.Expense{}).Error
}

func (r *expenseRepo) GetYears(userID int64) ([]int, error) {
	var years []int
	err := r.db.Model(&model.Expense{}).
		Where("user_id = ?", userID).
		Select("DISTINCT strftime('%Y', expense_date) as year").
		Order("year DESC").
		Pluck("year", &years).Error
	return years, err
}

func (r *expenseRepo) GetMonths(userID int64, year int) ([]int, error) {
	var months []int
	err := r.db.Model(&model.Expense{}).
		Where("user_id = ? AND strftime('%Y', expense_date) = ?", userID, fmt.Sprintf("%d", year)).
		Select("DISTINCT CAST(strftime('%m', expense_date) AS INTEGER) as month").
		Order("month DESC").
		Pluck("month", &months).Error
	return months, err
}

func (r *expenseRepo) GetByYearMonth(userID int64, year, month int) ([]*model.Expense, error) {
	var expenses []*model.Expense
	err := r.db.Where(
		"user_id = ? AND strftime('%Y', expense_date) = ? AND strftime('%m', expense_date) = ?",
		userID,
		fmt.Sprintf("%d", year),
		fmt.Sprintf("%02d", month),
	).Order("expense_date DESC, created_at DESC").Find(&expenses).Error
	return expenses, err
}

func (r *expenseRepo) Update(expense *model.Expense) error {
	return r.db.Model(expense).
		Where("id = ? AND user_id = ?", expense.ID, expense.UserID).
		Select("ledger_id", "type", "amount", "currency", "amount_in_base", "category", "description", "expense_date").
		Updates(expense).Error
}
