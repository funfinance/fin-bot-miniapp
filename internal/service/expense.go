package service

import (
	"fmt"
	"time"

	"fin-bot-miniapp/internal/logger"
	"fin-bot-miniapp/internal/model"
	"fin-bot-miniapp/internal/repository"
)

type ExpenseService struct {
	repo        repository.ExpenseRepository
	rateService *RateService
}

func NewExpenseService(repo repository.ExpenseRepository, rateService *RateService) *ExpenseService {
	return &ExpenseService{
		repo:        repo,
		rateService: rateService,
	}
}

func (s *ExpenseService) CreateExpense(userID int64, username string, ledgerID uint, recordType string, amount float64, currency, category, description string, expenseDate time.Time) (*model.Expense, error) {
	amountInBase, err := s.rateService.ConvertToBase(amount, currency)
	if err != nil {
		return nil, fmt.Errorf("convert currency: %w", err)
	}

	expense := &model.Expense{
		UserID:       userID,
		Username:     username,
		LedgerID:     ledgerID,
		Type:         recordType,
		Amount:       amount,
		Currency:     currency,
		AmountInBase: amountInBase,
		Category:     category,
		Description:  description,
		ExpenseDate:  expenseDate,
	}

	if err := s.repo.Create(expense); err != nil {
		return nil, fmt.Errorf("create %s: %w", recordType, err)
	}

	logger.Info("Created %s for user %d in ledger %d: %.2f %s (%s)", recordType, userID, ledgerID, amount, currency, category)
	return expense, nil
}

func (s *ExpenseService) GetMostRecentByUser(userID int64) (*model.Expense, error) {
	return s.repo.GetMostRecent(userID)
}

func (s *ExpenseService) DeleteExpense(id uint, userID int64) error {
	if err := s.repo.Delete(id, userID); err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	logger.Info("User %d deleted expense %d", userID, id)
	return nil
}

func (s *ExpenseService) GetRecentExpenses(userID int64, limit int) ([]*model.Expense, error) {
	expenses, err := s.repo.GetByUserID(userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get expenses: %w", err)
	}
	return expenses, nil
}

func (s *ExpenseService) GetExpensesByDateRange(userID int64, start, end time.Time) ([]*model.Expense, error) {
	expenses, err := s.repo.GetByUserIDAndDateRange(userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("get expenses by date range: %w", err)
	}
	return expenses, nil
}

func (s *ExpenseService) GetYears(userID int64) ([]int, error) {
	return s.repo.GetYears(userID)
}

func (s *ExpenseService) GetMonths(userID int64, year int) ([]int, error) {
	return s.repo.GetMonths(userID, year)
}

func (s *ExpenseService) GetByYearMonth(userID int64, year, month int) ([]*model.Expense, error) {
	return s.repo.GetByYearMonth(userID, year, month)
}

func (s *ExpenseService) UpdateExpense(userID int64, id uint, ledgerID uint, recordType, currency, category, description string, amount float64, expenseDate time.Time) (*model.Expense, error) {
	amountInBase, err := s.rateService.ConvertToBase(amount, currency)
	if err != nil {
		return nil, fmt.Errorf("convert currency: %w", err)
	}
	exp := &model.Expense{
		ID:           id,
		UserID:       userID,
		LedgerID:     ledgerID,
		Type:         recordType,
		Amount:       amount,
		Currency:     currency,
		AmountInBase: amountInBase,
		Category:     category,
		Description:  description,
		ExpenseDate:  expenseDate,
	}
	if err := s.repo.Update(exp); err != nil {
		return nil, fmt.Errorf("update expense: %w", err)
	}
	logger.Info("User %d updated expense %d", userID, id)
	return exp, nil
}

func (s *ExpenseService) GetExpensesByLedgerAndDateRange(userID int64, ledgerID uint, start, end time.Time) ([]*model.Expense, error) {
	var expenses []*model.Expense
	var err error

	if ledgerID == 0 {
		expenses, err = s.repo.GetByUserIDAndDateRange(userID, start, end)
	} else {
		expenses, err = s.repo.GetByUserIDLedgerIDAndDateRange(userID, ledgerID, start, end)
	}

	if err != nil {
		return nil, fmt.Errorf("get expenses by ledger and date range: %w", err)
	}

	return expenses, nil
}
