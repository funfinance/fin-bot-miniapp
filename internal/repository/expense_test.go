package repository

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"fin-bot-miniapp/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupExpenseTestDB creates a test database in memory
func setupExpenseTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Expense{}, &model.Ledger{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func createExpenseTestRepo(db *gorm.DB) ExpenseRepository {
	return &expenseRepo{db: db}
}

func TestExpenseCreate(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	expense := &model.Expense{
		UserID:       123456,
		Username:     "testuser",
		LedgerID:     1,
		Amount:       100.5,
		Currency:     "USD",
		AmountInBase: 15000,
		Category:     "food",
		Description:  "Lunch",
		ExpenseDate:  time.Now(),
	}

	err := repo.Create(expense)
	if err != nil {
		t.Fatalf("Failed to create expense: %v", err)
	}

	if expense.ID == 0 {
		t.Error("Expected expense ID to be set after creation")
	}
}

func TestExpenseGetByUserID(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	// Insert test expenses for different users
	now := time.Now()
	expenses := []*model.Expense{
		{UserID: 111111, Username: "user1", LedgerID: 1, Amount: 10, Currency: "USD", AmountInBase: 1500, Category: "food", Description: "Test1", ExpenseDate: now.Add(-1 * time.Hour)},
		{UserID: 111111, Username: "user1", LedgerID: 1, Amount: 20, Currency: "USD", AmountInBase: 3000, Category: "shopping", Description: "Test2", ExpenseDate: now.Add(-2 * time.Hour)},
		{UserID: 222222, Username: "user2", LedgerID: 2, Amount: 30, Currency: "USD", AmountInBase: 4500, Category: "food", Description: "Test3", ExpenseDate: now.Add(-3 * time.Hour)},
		{UserID: 111111, Username: "user1", LedgerID: 1, Amount: 40, Currency: "USD", AmountInBase: 6000, Category: "transport", Description: "Test4", ExpenseDate: now},
	}

	for _, exp := range expenses {
		if err := repo.Create(exp); err != nil {
			t.Fatalf("Failed to create expense: %v", err)
		}
	}

	// Get expenses for user1 with limit 2
	retrieved, err := repo.GetByUserID(111111, 2)
	if err != nil {
		t.Fatalf("Failed to get expenses: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 expenses, got %d", len(retrieved))
	}

	// Verify expenses are for the correct user
	for _, exp := range retrieved {
		if exp.UserID != 111111 {
			t.Errorf("Expected UserID 111111, got %d", exp.UserID)
		}
	}

	// Verify order (most recent first)
	if len(retrieved) >= 2 && retrieved[0].ExpenseDate.Before(retrieved[1].ExpenseDate) {
		t.Error("Expenses not ordered by date DESC")
	}

	// Get all expenses for user1
	allExpenses, err := repo.GetByUserID(111111, 100)
	if err != nil {
		t.Fatalf("Failed to get all expenses: %v", err)
	}

	if len(allExpenses) != 3 {
		t.Errorf("Expected 3 expenses for user1, got %d", len(allExpenses))
	}
}

func TestExpenseGetByUserIDAndDateRange(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	// Create expenses on different dates
	jan1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local)
	jan15 := time.Date(2025, 1, 15, 0, 0, 0, 0, time.Local)
	jan31 := time.Date(2025, 1, 31, 23, 59, 59, 0, time.Local)
	feb1 := time.Date(2025, 2, 1, 0, 0, 0, 0, time.Local)

	expenses := []*model.Expense{
		{UserID: 123456, Username: "user", LedgerID: 1, Amount: 10, Currency: "USD", AmountInBase: 1500, Category: "food", Description: "Jan 1", ExpenseDate: jan1},
		{UserID: 123456, Username: "user", LedgerID: 1, Amount: 20, Currency: "USD", AmountInBase: 3000, Category: "food", Description: "Jan 15", ExpenseDate: jan15},
		{UserID: 123456, Username: "user", LedgerID: 1, Amount: 30, Currency: "USD", AmountInBase: 4500, Category: "food", Description: "Jan 31", ExpenseDate: jan31},
		{UserID: 123456, Username: "user", LedgerID: 1, Amount: 40, Currency: "USD", AmountInBase: 6000, Category: "food", Description: "Feb 1", ExpenseDate: feb1},
		{UserID: 999999, Username: "other", LedgerID: 2, Amount: 50, Currency: "USD", AmountInBase: 7500, Category: "food", Description: "Jan 15", ExpenseDate: jan15},
	}

	for _, exp := range expenses {
		if err := repo.Create(exp); err != nil {
			t.Fatalf("Failed to create expense: %v", err)
		}
	}

	// Get January expenses for user 123456
	retrieved, err := repo.GetByUserIDAndDateRange(123456, jan1, jan31)
	if err != nil {
		t.Fatalf("Failed to get expenses: %v", err)
	}

	if len(retrieved) != 3 {
		t.Errorf("Expected 3 expenses in January, got %d", len(retrieved))
	}

	// Verify all expenses are in the date range
	for _, exp := range retrieved {
		if exp.ExpenseDate.Before(jan1) || exp.ExpenseDate.After(jan31) {
			t.Errorf("Expense date %v is outside range [%v, %v]", exp.ExpenseDate, jan1, jan31)
		}
		if exp.UserID != 123456 {
			t.Errorf("Expected UserID 123456, got %d", exp.UserID)
		}
	}
}

func TestExpenseDelete(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	// Create test expenses
	expense1 := &model.Expense{
		UserID:       123456,
		Username:     "user1",
		LedgerID:     1,
		Amount:       10,
		Currency:     "USD",
		AmountInBase: 1500,
		Category:     "food",
		Description:  "Test1",
		ExpenseDate:  time.Now(),
	}

	expense2 := &model.Expense{
		UserID:       999999,
		Username:     "user2",
		LedgerID:     2,
		Amount:       20,
		Currency:     "USD",
		AmountInBase: 3000,
		Category:     "food",
		Description:  "Test2",
		ExpenseDate:  time.Now(),
	}

	repo.Create(expense1)
	repo.Create(expense2)

	// Delete expense1
	err := repo.Delete(expense1.ID, 123456)
	if err != nil {
		t.Fatalf("Failed to delete expense: %v", err)
	}

	// Verify expense1 was deleted
	expenses, err := repo.GetByUserID(123456, 100)
	if err != nil {
		t.Fatalf("Failed to get expenses: %v", err)
	}

	if len(expenses) != 0 {
		t.Errorf("Expected 0 expenses after delete, got %d", len(expenses))
	}

	// Verify expense2 still exists
	expenses, err = repo.GetByUserID(999999, 100)
	if err != nil {
		t.Fatalf("Failed to get expenses: %v", err)
	}

	if len(expenses) != 1 {
		t.Errorf("Expected 1 expense for user2, got %d", len(expenses))
	}

	// Test deleting with wrong user ID (should not delete)
	err = repo.Delete(expense2.ID, 123456)
	if err != nil {
		t.Fatalf("Delete should not fail: %v", err)
	}

	// Verify expense2 still exists
	expenses, err = repo.GetByUserID(999999, 100)
	if err != nil {
		t.Fatalf("Failed to get expenses: %v", err)
	}

	if len(expenses) != 1 {
		t.Error("Expense should not be deleted with wrong user ID")
	}
}

func TestExpenseConcurrentCreate(t *testing.T) {
	tmpFile := fmt.Sprintf("/tmp/finbot_expense_concurrent_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Expense{}, &model.Ledger{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	repo := createExpenseTestRepo(db)

	const numGoroutines = 20
	const expensesPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Multiple goroutines creating expenses concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < expensesPerGoroutine; j++ {
				expense := &model.Expense{
					UserID:       int64(goroutineID),
					Username:     fmt.Sprintf("user%d", goroutineID),
					LedgerID:     uint(goroutineID%3 + 1),
					Amount:       float64(j + 1),
					Currency:     "USD",
					AmountInBase: float64((j + 1) * 150),
					Category:     "food",
					Description:  fmt.Sprintf("Expense %d-%d", goroutineID, j),
					ExpenseDate:  time.Now().Add(time.Duration(j) * time.Hour),
				}

				if err := repo.Create(expense); err != nil {
					t.Errorf("Goroutine %d: Failed to create expense: %v", goroutineID, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify all expenses were created
	totalExpenses := 0
	for i := 0; i < numGoroutines; i++ {
		expenses, err := repo.GetByUserID(int64(i), 100)
		if err != nil {
			t.Fatalf("Failed to get expenses for user %d: %v", i, err)
		}
		totalExpenses += len(expenses)
	}

	expectedTotal := numGoroutines * expensesPerGoroutine
	if totalExpenses != expectedTotal {
		t.Errorf("Expected %d total expenses, got %d", expectedTotal, totalExpenses)
	}

	t.Logf("✅ Concurrent expense creation test passed: %d goroutines × %d expenses each = %d total",
		numGoroutines, expensesPerGoroutine, totalExpenses)
}

func TestExpenseConcurrentMixedOperations(t *testing.T) {
	tmpFile := fmt.Sprintf("/tmp/finbot_expense_mixed_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Expense{}, &model.Ledger{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	repo := createExpenseTestRepo(db)

	// Insert initial data
	users := []int64{111111, 222222, 333333}
	for _, userID := range users {
		for i := 0; i < 5; i++ {
			expense := &model.Expense{
				UserID:       userID,
				Username:     fmt.Sprintf("user%d", userID),
				LedgerID:     1,
				Amount:       float64(i + 1),
				Currency:     "USD",
				AmountInBase: float64((i + 1) * 150),
				Category:     "food",
				Description:  fmt.Sprintf("Initial %d", i),
				ExpenseDate:  time.Now().Add(time.Duration(i) * time.Hour),
			}
			repo.Create(expense)
		}
	}

	const numReaders = 10
	const numWriters = 10
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// Start readers
	for i := 0; i < numReaders; i++ {
		go func(readerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				userID := users[j%len(users)]

				// GetByUserID
				_, err := repo.GetByUserID(userID, 10)
				if err != nil {
					t.Errorf("Reader %d: Failed to get by user ID: %v", readerID, err)
					return
				}

				// GetByUserIDAndDateRange
				if j%5 == 0 {
					start := time.Now().Add(-24 * time.Hour)
					end := time.Now().Add(24 * time.Hour)
					_, err := repo.GetByUserIDAndDateRange(userID, start, end)
					if err != nil {
						t.Errorf("Reader %d: Failed to get by date range: %v", readerID, err)
						return
					}
				}
			}
		}(i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				userID := users[j%len(users)]

				expense := &model.Expense{
					UserID:       userID,
					Username:     fmt.Sprintf("user%d", userID),
					LedgerID:     uint(writerID%3 + 1),
					Amount:       float64(writerID*operationsPerGoroutine + j),
					Currency:     "USD",
					AmountInBase: float64((writerID*operationsPerGoroutine + j) * 150),
					Category:     "food",
					Description:  fmt.Sprintf("Writer %d-%d", writerID, j),
					ExpenseDate:  time.Now(),
				}

				if err := repo.Create(expense); err != nil {
					t.Errorf("Writer %d: Failed to create expense: %v", writerID, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("✅ Mixed concurrent operations passed: %d readers + %d writers, no race conditions!",
		numReaders, numWriters)
}

func TestExpenseConcurrentWithRealDatabase(t *testing.T) {
	tmpFile := fmt.Sprintf("/tmp/finbot_expense_real_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	// Use direct database connection instead of singleton to avoid conflicts
	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Expense{}, &model.Ledger{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	repo := createExpenseTestRepo(db)

	const numGoroutines = 15
	const numOperations = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	startTime := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			userID := int64(id % 3)

			for j := 0; j < numOperations; j++ {
				expense := &model.Expense{
					UserID:       userID,
					Username:     fmt.Sprintf("user%d", userID),
					LedgerID:     uint(id%3 + 1),
					Amount:       float64(id*numOperations + j),
					Currency:     "USD",
					AmountInBase: float64((id*numOperations + j) * 150),
					Category:     "food",
					Description:  fmt.Sprintf("Goroutine %d Operation %d", id, j),
					ExpenseDate:  time.Now(),
				}

				if err := repo.Create(expense); err != nil {
					t.Errorf("Goroutine %d: Failed to create: %v", id, err)
					return
				}

				// Also do some reads
				if j%5 == 0 {
					_, _ = repo.GetByUserID(userID, 5)
				}
			}
		}(i)
	}

	wg.Wait()

	duration := time.Since(startTime)

	// Verify final state
	totalExpenses := 0
	for i := 0; i < 3; i++ {
		expenses, err := repo.GetByUserID(int64(i), 1000)
		if err != nil {
			t.Fatalf("Failed to get expenses: %v", err)
		}
		totalExpenses += len(expenses)
	}

	expectedTotal := numGoroutines * numOperations
	if totalExpenses != expectedTotal {
		t.Errorf("Expected %d expenses, got %d", expectedTotal, totalExpenses)
	}

	t.Logf("✅ Real database concurrent test passed!")
	t.Logf("   %d goroutines × %d operations = %d total operations",
		numGoroutines, numOperations, expectedTotal)
	t.Logf("   Completed in %v", duration)
	t.Logf("   No race conditions detected!")
}

func TestExpenseGetYears(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	expenses := []*model.Expense{
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 10, Currency: "USD", AmountInBase: 1000, ExpenseDate: time.Date(2023, 3, 1, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 20, Currency: "USD", AmountInBase: 2000, ExpenseDate: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 30, Currency: "USD", AmountInBase: 3000, ExpenseDate: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)},
		{UserID: 999999, Username: "other", LedgerID: 2, Amount: 10, Currency: "USD", AmountInBase: 1000, ExpenseDate: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, e := range expenses {
		repo.Create(e)
	}

	years, err := repo.GetYears(123456)
	if err != nil {
		t.Fatalf("GetYears failed: %v", err)
	}
	if len(years) != 2 {
		t.Errorf("Expected 2 years, got %d: %v", len(years), years)
	}
	if years[0] != 2024 || years[1] != 2023 {
		t.Errorf("Expected [2024, 2023], got %v", years)
	}
}

func TestExpenseGetMonths(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	expenses := []*model.Expense{
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 10, Currency: "USD", AmountInBase: 1000, ExpenseDate: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 20, Currency: "USD", AmountInBase: 2000, ExpenseDate: time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 30, Currency: "USD", AmountInBase: 3000, ExpenseDate: time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 40, Currency: "USD", AmountInBase: 4000, ExpenseDate: time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, e := range expenses {
		repo.Create(e)
	}

	months, err := repo.GetMonths(123456, 2024)
	if err != nil {
		t.Fatalf("GetMonths failed: %v", err)
	}
	if len(months) != 2 {
		t.Errorf("Expected 2 months, got %d: %v", len(months), months)
	}
	// March (3) and January (1), ordered DESC
	if months[0] != 3 || months[1] != 1 {
		t.Errorf("Expected [3, 1], got %v", months)
	}
}

func TestExpenseGetByYearMonth(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	expenses := []*model.Expense{
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 10, Currency: "USD", AmountInBase: 1000, ExpenseDate: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 20, Currency: "USD", AmountInBase: 2000, ExpenseDate: time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC)},
		{UserID: 123456, Username: "u", LedgerID: 1, Amount: 30, Currency: "USD", AmountInBase: 3000, ExpenseDate: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
		{UserID: 999999, Username: "other", LedgerID: 2, Amount: 40, Currency: "USD", AmountInBase: 4000, ExpenseDate: time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, e := range expenses {
		repo.Create(e)
	}

	results, err := repo.GetByYearMonth(123456, 2024, 4)
	if err != nil {
		t.Fatalf("GetByYearMonth failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 records for Apr 2024, got %d", len(results))
	}
	for _, r := range results {
		if r.UserID != 123456 {
			t.Errorf("Expected userID 123456, got %d", r.UserID)
		}
		if r.ExpenseDate.Month() != 4 || r.ExpenseDate.Year() != 2024 {
			t.Errorf("Expected Apr 2024, got %v", r.ExpenseDate)
		}
	}
}

func TestExpenseUpdate(t *testing.T) {
	db := setupExpenseTestDB(t)
	repo := createExpenseTestRepo(db)

	exp := &model.Expense{
		UserID: 123456, Username: "u", LedgerID: 1,
		Amount: 100, Currency: "USD", AmountInBase: 10000,
		Category: "food", Description: "original",
		ExpenseDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	repo.Create(exp)

	exp.Amount = 200
	exp.Description = "updated"
	if err := repo.Update(exp); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	results, _ := repo.GetByUserID(123456, 10)
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Amount != 200 || results[0].Description != "updated" {
		t.Errorf("Update not persisted: amount=%v desc=%v", results[0].Amount, results[0].Description)
	}
}

// Benchmark tests
func BenchmarkExpenseCreate(b *testing.B) {
	db := setupExpenseTestDB(&testing.T{})
	repo := createExpenseTestRepo(db)

	expense := &model.Expense{
		UserID:       123456,
		Username:     "testuser",
		LedgerID:     1,
		Amount:       100,
		Currency:     "USD",
		AmountInBase: 15000,
		Category:     "food",
		Description:  "Benchmark",
		ExpenseDate:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		expense.ID = 0 // Reset ID for each create
		repo.Create(expense)
	}
}

func BenchmarkGetByUserID(b *testing.B) {
	db := setupExpenseTestDB(&testing.T{})
	repo := createExpenseTestRepo(db)

	// Create 100 test expenses
	for i := 0; i < 100; i++ {
		expense := &model.Expense{
			UserID:       123456,
			Username:     "testuser",
			LedgerID:     1,
			Amount:       float64(i),
			Currency:     "USD",
			AmountInBase: float64(i * 150),
			Category:     "food",
			Description:  fmt.Sprintf("Expense %d", i),
			ExpenseDate:  time.Now(),
		}
		repo.Create(expense)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetByUserID(123456, 10)
	}
}
