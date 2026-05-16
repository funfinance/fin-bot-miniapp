package service

import (
	"fmt"
	"testing"
	"time"

	"fin-bot-miniapp/internal/constant"
	"fin-bot-miniapp/internal/model"
)

// mockExpenseRepo is used by expense service tests
type mockExpenseRepo struct {
	expenses []*model.Expense
	nextID   uint
}

func newMockExpenseRepo() *mockExpenseRepo {
	return &mockExpenseRepo{
		expenses: make([]*model.Expense, 0),
		nextID:   1,
	}
}

func (m *mockExpenseRepo) Create(expense *model.Expense) error {
	expense.ID = m.nextID
	m.nextID++
	expense.CreatedAt = time.Now()
	m.expenses = append(m.expenses, expense)
	return nil
}

func (m *mockExpenseRepo) GetByUserID(userID int64, limit int) ([]*model.Expense, error) {
	var result []*model.Expense
	count := 0

	for i := len(m.expenses) - 1; i >= 0 && count < limit; i-- {
		if m.expenses[i].UserID == userID {
			result = append(result, m.expenses[i])
			count++
		}
	}

	return result, nil
}

func (m *mockExpenseRepo) GetByUserIDAndDateRange(userID int64, start, end time.Time) ([]*model.Expense, error) {
	var result []*model.Expense

	for _, expense := range m.expenses {
		if expense.UserID == userID &&
			!expense.ExpenseDate.Before(start) &&
			!expense.ExpenseDate.After(end) {
			result = append(result, expense)
		}
	}

	return result, nil
}

func (m *mockExpenseRepo) GetByUserIDLedgerIDAndDateRange(userID int64, ledgerID uint, start, end time.Time) ([]*model.Expense, error) {
	var result []*model.Expense

	for _, expense := range m.expenses {
		if expense.UserID == userID &&
			expense.LedgerID == ledgerID &&
			!expense.ExpenseDate.Before(start) &&
			!expense.ExpenseDate.After(end) {
			result = append(result, expense)
		}
	}

	return result, nil
}

func (m *mockExpenseRepo) GetMostRecent(userID int64) (*model.Expense, error) {
	var latest *model.Expense
	for _, exp := range m.expenses {
		if exp.UserID == userID {
			if latest == nil || exp.CreatedAt.After(latest.CreatedAt) {
				e := exp
				latest = e
			}
		}
	}
	if latest == nil {
		return nil, fmt.Errorf("record not found")
	}
	return latest, nil
}

func (m *mockExpenseRepo) Delete(id uint, userID int64) error {
	for i, exp := range m.expenses {
		if exp.ID == id && exp.UserID == userID {
			m.expenses = append(m.expenses[:i], m.expenses[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockExpenseRepo) GetYears(userID int64) ([]int, error) {
	seen := map[int]bool{}
	var years []int
	for _, exp := range m.expenses {
		if exp.UserID == userID {
			y := exp.ExpenseDate.Year()
			if !seen[y] {
				seen[y] = true
				years = append(years, y)
			}
		}
	}
	return years, nil
}

func (m *mockExpenseRepo) GetMonths(userID int64, year int) ([]int, error) {
	seen := map[int]bool{}
	var months []int
	for _, exp := range m.expenses {
		if exp.UserID == userID && exp.ExpenseDate.Year() == year {
			mo := int(exp.ExpenseDate.Month())
			if !seen[mo] {
				seen[mo] = true
				months = append(months, mo)
			}
		}
	}
	return months, nil
}

func (m *mockExpenseRepo) GetByYearMonth(userID int64, year, month int) ([]*model.Expense, error) {
	var result []*model.Expense
	for _, exp := range m.expenses {
		if exp.UserID == userID && exp.ExpenseDate.Year() == year && int(exp.ExpenseDate.Month()) == month {
			result = append(result, exp)
		}
	}
	return result, nil
}

func (m *mockExpenseRepo) Update(expense *model.Expense) error {
	for i, exp := range m.expenses {
		if exp.ID == expense.ID && exp.UserID == expense.UserID {
			m.expenses[i] = expense
			return nil
		}
	}
	return fmt.Errorf("record not found")
}

// mockExpenseRateRepo is used by expense service tests (rate repo mock)
type mockExpenseRateRepo struct {
	rates map[string]float64
}

func newMockExpenseRateRepo() *mockExpenseRateRepo {
	return &mockExpenseRateRepo{
		rates: map[string]float64{
			"USD": 0.0067,
			"CNY": 0.048,
			"EUR": 0.0055,
		},
	}
}

func (m *mockExpenseRateRepo) Upsert(rate *model.ExchangeRate) error {
	m.rates[rate.Currency] = rate.RateToBase
	return nil
}

func (m *mockExpenseRateRepo) GetByCurrency(currency string) (*model.ExchangeRate, error) {
	if rateValue, exists := m.rates[currency]; exists {
		return &model.ExchangeRate{
			Currency:   currency,
			RateToBase: rateValue,
		}, nil
	}
	return nil, ErrRateNotFound
}

func (m *mockExpenseRateRepo) GetAll() ([]*model.ExchangeRate, error) {
	var rates []*model.ExchangeRate
	for currency, rateValue := range m.rates {
		rates = append(rates, &model.ExchangeRate{
			Currency:   currency,
			RateToBase: rateValue,
		})
	}
	return rates, nil
}

func TestCreateExpense(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD", "CNY"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	_, err := service.CreateExpense(
		123456,
		"testuser",
		1,
		constant.TypeExpense,
		100.0,
		"USD",
		"food",
		"Test expense",
		time.Now(),
	)

	if err != nil {
		t.Fatalf("Failed to create expense: %v", err)
	}

	if len(expenseRepo.expenses) != 1 {
		t.Fatalf("Expected 1 expense, got %d", len(expenseRepo.expenses))
	}

	expense := expenseRepo.expenses[0]

	if expense.UserID != 123456 {
		t.Errorf("Expected UserID 123456, got %d", expense.UserID)
	}

	if expense.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", expense.Username)
	}

	if expense.Amount != 100.0 {
		t.Errorf("Expected amount 100.0, got %.2f", expense.Amount)
	}

	if expense.Currency != "USD" {
		t.Errorf("Expected currency 'USD', got '%s'", expense.Currency)
	}

	if expense.Category != "food" {
		t.Errorf("Expected category 'food', got '%s'", expense.Category)
	}

	expectedJPY := 100.0 / 0.0067
	diff := expense.AmountInBase - expectedJPY
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.1 {
		t.Errorf("Expected amount in JPY ~%.2f, got %.2f", expectedJPY, expense.AmountInBase)
	}
}

func TestCreateExpenseWithDifferentCurrencies(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD", "CNY", "EUR"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)
	rateService.UpdateRate("CNY", 0.048)
	rateService.UpdateRate("EUR", 0.0055)

	tests := []struct {
		amount      float64
		currency    string
		expectedJPY float64
	}{
		{100, "USD", 14925.37},
		{100, "CNY", 2083.33},
		{100, "EUR", 18181.82},
	}

	for _, tt := range tests {
		_, err := service.CreateExpense(
			123456,
			"testuser",
			1,
			constant.TypeExpense,
			tt.amount,
			tt.currency,
			"food",
			"Test",
			time.Now(),
		)

		if err != nil {
			t.Errorf("Failed to create expense with %s: %v", tt.currency, err)
			continue
		}
	}

	if len(expenseRepo.expenses) != len(tests) {
		t.Errorf("Expected %d expenses, got %d", len(tests), len(expenseRepo.expenses))
	}

	for i, tt := range tests {
		expense := expenseRepo.expenses[i]
		diff := expense.AmountInBase - tt.expectedJPY
		if diff < 0 {
			diff = -diff
		}
		if diff > 1.0 {
			t.Errorf("Test %d (%s): expected JPY %.2f, got %.2f",
				i, tt.currency, tt.expectedJPY, expense.AmountInBase)
		}
	}
}

func TestGetRecentExpenses(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	for i := 0; i < 15; i++ {
		service.CreateExpense(
			123456,
			"testuser",
			1,
			constant.TypeExpense,
			float64(i+1)*10,
			"USD",
			"food",
			"Test",
			time.Now().Add(time.Duration(i)*time.Hour),
		)
	}

	expenses, err := service.GetRecentExpenses(123456, 10)
	if err != nil {
		t.Fatalf("Failed to get recent expenses: %v", err)
	}

	if len(expenses) != 10 {
		t.Errorf("Expected 10 expenses, got %d", len(expenses))
	}

	for i := 0; i < len(expenses)-1; i++ {
		if expenses[i].Amount < expenses[i+1].Amount {
			t.Error("Expenses should be ordered from most recent to oldest")
			break
		}
	}
}

func TestGetRecentExpensesWithLimit(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	for i := 0; i < 5; i++ {
		service.CreateExpense(123456, "testuser", 1, constant.TypeExpense, 10, "USD", "food", "Test", time.Now())
	}

	expenses, err := service.GetRecentExpenses(123456, 10)
	if err != nil {
		t.Fatalf("Failed to get recent expenses: %v", err)
	}

	if len(expenses) != 5 {
		t.Errorf("Expected 5 expenses (not 10), got %d", len(expenses))
	}
}

func TestGetRecentExpensesMultipleUsers(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	for i := 0; i < 5; i++ {
		service.CreateExpense(111111, "user1", 1, constant.TypeExpense, 10, "USD", "food", "User1", time.Now())
		service.CreateExpense(222222, "user2", 2, constant.TypeExpense, 20, "USD", "food", "User2", time.Now())
	}

	expenses, err := service.GetRecentExpenses(111111, 10)
	if err != nil {
		t.Fatalf("Failed to get expenses for user1: %v", err)
	}

	if len(expenses) != 5 {
		t.Errorf("Expected 5 expenses for user1, got %d", len(expenses))
	}

	for _, expense := range expenses {
		if expense.UserID != 111111 {
			t.Errorf("Expected UserID 111111, got %d", expense.UserID)
		}
	}

	expenses, err = service.GetRecentExpenses(222222, 10)
	if err != nil {
		t.Fatalf("Failed to get expenses for user2: %v", err)
	}

	if len(expenses) != 5 {
		t.Errorf("Expected 5 expenses for user2, got %d", len(expenses))
	}

	for _, expense := range expenses {
		if expense.UserID != 222222 {
			t.Errorf("Expected UserID 222222, got %d", expense.UserID)
		}
	}
}

func TestGetExpensesByDateRange(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	jan1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	jan15 := time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local)
	jan31 := time.Date(2025, 1, 31, 23, 59, 59, 0, time.Local)
	feb1 := time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)

	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 10, "USD", "food", "Jan 1", jan1)
	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 20, "USD", "food", "Jan 15", jan15)
	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 30, "USD", "food", "Jan 31", jan31)
	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 40, "USD", "food", "Feb 1", feb1)

	expenses, err := service.GetExpensesByDateRange(123456, jan1, jan31)
	if err != nil {
		t.Fatalf("Failed to get expenses by date range: %v", err)
	}

	if len(expenses) != 3 {
		t.Errorf("Expected 3 expenses in January, got %d", len(expenses))
	}

	for _, expense := range expenses {
		if expense.ExpenseDate.After(jan31) {
			t.Errorf("Expense date %v is after Jan 31", expense.ExpenseDate)
		}
		if expense.ExpenseDate.Before(jan1) {
			t.Errorf("Expense date %v is before Jan 1", expense.ExpenseDate)
		}
	}

	expenses, err = service.GetExpensesByDateRange(123456, feb1, feb1.Add(30*24*time.Hour))
	if err != nil {
		t.Fatalf("Failed to get February expenses: %v", err)
	}

	if len(expenses) != 1 {
		t.Errorf("Expected 1 expense in February, got %d", len(expenses))
	}
}

func TestGetExpensesByDateRangeMultipleUsers(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	jan1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	jan31 := time.Date(2025, 1, 31, 23, 59, 59, 0, time.Local)

	service.CreateExpense(111111, "user1", 1, constant.TypeExpense, 10, "USD", "food", "User1", jan1)
	service.CreateExpense(222222, "user2", 2, constant.TypeExpense, 20, "USD", "food", "User2", jan1.Add(1*time.Hour))

	expenses, err := service.GetExpensesByDateRange(111111, jan1, jan31)
	if err != nil {
		t.Fatalf("Failed to get expenses: %v", err)
	}

	if len(expenses) != 1 {
		t.Errorf("Expected 1 expense for user1, got %d", len(expenses))
	}

	if expenses[0].UserID != 111111 {
		t.Errorf("Expected UserID 111111, got %d", expenses[0].UserID)
	}
}

func TestCreateExpenseWithIncomeType(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	_, err := service.CreateExpense(
		123456,
		"testuser",
		1,
		constant.TypeIncome,
		1000.0,
		"USD",
		"Salary",
		"Monthly salary",
		time.Now(),
	)

	if err != nil {
		t.Fatalf("Failed to create income: %v", err)
	}

	if len(expenseRepo.expenses) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(expenseRepo.expenses))
	}

	record := expenseRepo.expenses[0]

	if record.Type != constant.TypeIncome {
		t.Errorf("Expected Type '%s', got '%s'", constant.TypeIncome, record.Type)
	}

	if record.Category != "Salary" {
		t.Errorf("Expected category 'Salary', got '%s'", record.Category)
	}
}

func TestCreateMixedIncomeAndExpense(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	service.CreateExpense(123456, "user", 1, constant.TypeIncome, 1000, "USD", "Salary", "Income", time.Now())
	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 100, "USD", "Food", "Lunch", time.Now())

	if len(expenseRepo.expenses) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(expenseRepo.expenses))
	}

	if expenseRepo.expenses[0].Type != constant.TypeIncome {
		t.Errorf("Expected first record to be income, got %s", expenseRepo.expenses[0].Type)
	}

	if expenseRepo.expenses[1].Type != constant.TypeExpense {
		t.Errorf("Expected second record to be expense, got %s", expenseRepo.expenses[1].Type)
	}
}

func TestCreateExpenseWithInvalidCurrency(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	_, err := service.CreateExpense(
		123456,
		"testuser",
		1,
		constant.TypeExpense,
		100,
		"INVALID",
		"food",
		"Test",
		time.Now(),
	)

	if err == nil {
		t.Error("Expected error for invalid currency, got nil")
	}

	if len(expenseRepo.expenses) != 0 {
		t.Errorf("Expected 0 expenses after error, got %d", len(expenseRepo.expenses))
	}
}

func TestGetMostRecentByUser(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	_, err := service.GetMostRecentByUser(123456)
	if err == nil {
		t.Error("Expected error when no records exist, got nil")
	}

	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 2, 10, 0, 0, 0, time.UTC)
	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 10, "USD", "food", "older", t1)
	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 20, "USD", "food", "newer", t2)

	record, err := service.GetMostRecentByUser(123456)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if record.Amount != 20 {
		t.Errorf("Expected most recent amount 20, got %.2f", record.Amount)
	}

	_, err = service.GetMostRecentByUser(999999)
	if err == nil {
		t.Error("Expected error for user with no records, got nil")
	}
}

func TestDeleteExpense(t *testing.T) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	service.CreateExpense(123456, "user", 1, constant.TypeExpense, 50, "USD", "food", "test", time.Now())

	if len(expenseRepo.expenses) != 1 {
		t.Fatalf("Expected 1 expense before delete, got %d", len(expenseRepo.expenses))
	}

	id := expenseRepo.expenses[0].ID

	service.DeleteExpense(id, 999999)
	if len(expenseRepo.expenses) != 1 {
		t.Error("Record should not be deleted when userID does not match")
	}

	err := service.DeleteExpense(id, 123456)
	if err != nil {
		t.Fatalf("Unexpected error on delete: %v", err)
	}
	if len(expenseRepo.expenses) != 0 {
		t.Errorf("Expected 0 expenses after delete, got %d", len(expenseRepo.expenses))
	}
}

func BenchmarkCreateExpense(b *testing.B) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.CreateExpense(123456, "user", 1, constant.TypeExpense, 100, "USD", "food", "Test", time.Now())
	}
}

func BenchmarkGetRecentExpenses(b *testing.B) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	for i := 0; i < 100; i++ {
		service.CreateExpense(123456, "user", 1, constant.TypeExpense, 100, "USD", "food", "Test", time.Now())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetRecentExpenses(123456, 10)
	}
}

func BenchmarkGetExpensesByDateRange(b *testing.B) {
	expenseRepo := newMockExpenseRepo()
	rateRepo := newMockExpenseRateRepo()
	rateService := NewRateService(rateRepo, 24*time.Hour, "", "JPY", []string{"USD"})
	service := NewExpenseService(expenseRepo, rateService)

	rateService.UpdateRate("USD", 0.0067)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 100; i++ {
		service.CreateExpense(123456, "user", 1, constant.TypeExpense, 100, "USD", "food", "Test", start.Add(time.Duration(i)*time.Hour))
	}

	end := start.Add(100 * time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetExpensesByDateRange(123456, start, end)
	}
}
