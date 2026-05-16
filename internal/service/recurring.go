package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"fin-bot-miniapp/internal/logger"
	"fin-bot-miniapp/internal/model"
	"fin-bot-miniapp/internal/repository"
)

type RecurringService struct {
	repo        repository.RecurringExpenseRepository
	expenseRepo repository.ExpenseRepository
	rateService *RateService
}

func NewRecurringService(repo repository.RecurringExpenseRepository, expenseRepo repository.ExpenseRepository, rateService *RateService) *RecurringService {
	return &RecurringService{
		repo:        repo,
		expenseRepo: expenseRepo,
		rateService: rateService,
	}
}

func (s *RecurringService) Create(r *model.RecurringExpense, initialDate time.Time) error {
	now := time.Now()

	next, err := nextTrigger(r.Frequency, r.Days, now)
	if err != nil {
		return fmt.Errorf("calculate next trigger: %w", err)
	}
	r.NextTriggerAt = next
	r.LastTriggeredAt = &initialDate

	if err := s.repo.Create(r); err != nil {
		return err
	}

	if err := s.createExpense(r, initialDate); err != nil {
		return err
	}

	logger.Info("Created recurring for user %d: %.2f %s every %s (days: %s), next: %s",
		r.UserID, r.Amount, r.Currency, r.Frequency, r.Days, r.NextTriggerAt.Format("2006-01-02"))
	return nil
}

// Start runs the daily scheduler. Blocks until ctx is cancelled.
func (s *RecurringService) Start(ctx context.Context) {
	logger.Info("RecurringService: scheduler started")
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		logger.Info("RecurringService: next scan at %s", next.Format("2006-01-02 15:04:05"))
		select {
		case <-ctx.Done():
			logger.Info("RecurringService: scheduler stopped")
			return
		case <-time.After(time.Until(next)):
			s.runOnce(time.Now())
		}
	}
}

func (s *RecurringService) runOnce(now time.Time) {
	due, err := s.repo.GetDue(now)
	if err != nil {
		logger.Error("RecurringService: fetch due records: %v", err)
		return
	}
	if len(due) > 0 {
		logger.Info("RecurringService: daily scan — %d due record(s)", len(due))
	}
	for _, r := range due {
		if err := s.trigger(r, now); err != nil {
			logger.Error("RecurringService: trigger id=%d: %v", r.ID, err)
		}
	}
}

func (s *RecurringService) trigger(r *model.RecurringExpense, now time.Time) error {
	if err := s.createExpense(r, now); err != nil {
		return err
	}

	next, err := nextTrigger(r.Frequency, r.Days, now)
	if err != nil {
		return fmt.Errorf("calculate next trigger: %w", err)
	}
	if err := s.repo.UpdateTrigger(r.ID, now, next); err != nil {
		return fmt.Errorf("update trigger: %w", err)
	}

	logger.Info("RecurringService: triggered id=%d user=%d amount=%.2f %s next=%s", r.ID, r.UserID, r.Amount, r.Currency, next.Format("2006-01-02"))
	return nil
}

func (s *RecurringService) createExpense(r *model.RecurringExpense, now time.Time) error {
	amountInBase, err := s.rateService.ConvertToBase(r.Amount, r.Currency)
	if err != nil {
		return fmt.Errorf("convert currency: %w", err)
	}
	expense := &model.Expense{
		UserID:       r.UserID,
		Username:     r.Username,
		LedgerID:     r.LedgerID,
		Type:         r.Type,
		Amount:       r.Amount,
		Currency:     r.Currency,
		AmountInBase: amountInBase,
		Category:     r.Category,
		Description:  r.Description,
		ExpenseDate:  now,
	}
	if err := s.expenseRepo.Create(expense); err != nil {
		return fmt.Errorf("create expense: %w", err)
	}
	return nil
}

// nextTrigger returns the next trigger time after `after`.
// frequency: "weekly" or "monthly"
// days: comma-separated ints (weekday 0-6 or month day 1-31)
func nextTrigger(frequency, days string, after time.Time) (time.Time, error) {
	parsed, err := parseDays(days)
	if err != nil {
		return time.Time{}, err
	}

	switch frequency {
	case "weekly":
		return nextWeeklyTrigger(parsed, after), nil
	case "monthly":
		return nextMonthlyTrigger(parsed, after), nil
	default:
		return time.Time{}, fmt.Errorf("unknown frequency: %s", frequency)
	}
}

func nextWeeklyTrigger(weekdays []int, after time.Time) time.Time {
	sort.Ints(weekdays)
	afterWeekday := int(after.Weekday())
	for _, wd := range weekdays {
		if wd > afterWeekday {
			return midnight(after.AddDate(0, 0, wd-afterWeekday))
		}
	}
	// wrap to next week
	daysToNextSunday := 7 - afterWeekday
	return midnight(after.AddDate(0, 0, daysToNextSunday+weekdays[0]))
}

func nextMonthlyTrigger(monthDays []int, after time.Time) time.Time {
	sort.Ints(monthDays)
	afterDay := after.Day()
	for _, d := range monthDays {
		if d > afterDay {
			return clampedMonthDay(after.Year(), after.Month(), d)
		}
	}
	// wrap to next month
	nextMonth := after.Month() + 1
	nextYear := after.Year()
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}
	return clampedMonthDay(nextYear, nextMonth, monthDays[0])
}

// clampedMonthDay returns midnight of year/month/day, clamping to last day of month if needed.
func clampedMonthDay(year int, month time.Month, day int) time.Time {
	lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	if day > lastDay {
		day = lastDay
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.Local)
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func parseDays(days string) ([]int, error) {
	parts := strings.Split(days, ",")
	var result []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid day value %q", p)
		}
		result = append(result, n)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("days must not be empty")
	}
	return result, nil
}
