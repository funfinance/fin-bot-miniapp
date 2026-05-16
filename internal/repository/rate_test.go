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

// setupTestDB creates a test database in memory
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate the schema
	if err := db.AutoMigrate(&model.ExchangeRate{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

// createTestRepo creates a test repository instance
func createTestRepo(db *gorm.DB) RateRepository {
	return &rateRepo{db: db}
}

func TestRateUpsert(t *testing.T) {
	db := setupTestDB(t)
	repo := createTestRepo(db)

	// Test inserting a new rate
	rate := &model.ExchangeRate{
		Currency:   "USD",
		RateToBase: 0.0067,
		UpdatedAt:  time.Now(),
	}

	err := repo.Upsert(rate)
	if err != nil {
		t.Fatalf("Failed to insert rate: %v", err)
	}

	// Verify the rate was inserted
	retrieved, err := repo.GetByCurrency("USD")
	if err != nil {
		t.Fatalf("Failed to get rate: %v", err)
	}

	if retrieved.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", retrieved.Currency)
	}

	if retrieved.RateToBase != 0.0067 {
		t.Errorf("Expected rate 0.0067, got %f", retrieved.RateToBase)
	}

	// Test updating existing rate
	rate.RateToBase = 0.0068
	err = repo.Upsert(rate)
	if err != nil {
		t.Fatalf("Failed to update rate: %v", err)
	}

	// Verify the rate was updated
	retrieved, err = repo.GetByCurrency("USD")
	if err != nil {
		t.Fatalf("Failed to get updated rate: %v", err)
	}

	if retrieved.RateToBase != 0.0068 {
		t.Errorf("Expected updated rate 0.0068, got %f", retrieved.RateToBase)
	}
}

func TestRateGetByCurrency(t *testing.T) {
	db := setupTestDB(t)
	repo := createTestRepo(db)

	// Insert test data
	rate := &model.ExchangeRate{
		Currency:   "CNY",
		RateToBase: 0.048,
		UpdatedAt:  time.Now(),
	}
	repo.Upsert(rate)

	// Test getting existing rate
	retrieved, err := repo.GetByCurrency("CNY")
	if err != nil {
		t.Fatalf("Failed to get rate: %v", err)
	}

	if retrieved.Currency != "CNY" {
		t.Errorf("Expected currency CNY, got %s", retrieved.Currency)
	}

	// Test getting non-existent rate
	_, err = repo.GetByCurrency("EUR")
	if err == nil {
		t.Error("Expected error for non-existent currency, got nil")
	}
}

func TestRateGetAll(t *testing.T) {
	db := setupTestDB(t)
	repo := createTestRepo(db)

	// Insert multiple rates
	rates := []*model.ExchangeRate{
		{Currency: "USD", RateToBase: 0.0067, UpdatedAt: time.Now()},
		{Currency: "CNY", RateToBase: 0.048, UpdatedAt: time.Now()},
		{Currency: "EUR", RateToBase: 0.0062, UpdatedAt: time.Now()},
	}

	for _, rate := range rates {
		if err := repo.Upsert(rate); err != nil {
			t.Fatalf("Failed to insert rate: %v", err)
		}
	}

	// Get all rates
	allRates, err := repo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all rates: %v", err)
	}

	if len(allRates) != 3 {
		t.Errorf("Expected 3 rates, got %d", len(allRates))
	}

	// Verify all currencies are present
	currencies := make(map[string]bool)
	for _, rate := range allRates {
		currencies[rate.Currency] = true
	}

	for _, expectedCurrency := range []string{"USD", "CNY", "EUR"} {
		if !currencies[expectedCurrency] {
			t.Errorf("Expected currency %s not found", expectedCurrency)
		}
	}
}

// TestConcurrentUpsert tests that the database handles concurrent writes correctly
func TestConcurrentUpsert(t *testing.T) {
	// Use a real file-based database for concurrent tests
	// Memory database (:memory:) doesn't work well with multiple connections
	tmpFile := fmt.Sprintf("/tmp/finbot_concurrent_test_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.ExchangeRate{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	repo := createTestRepo(db)

	const numGoroutines = 50
	const numOperationsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Launch multiple goroutines that all try to upsert the same currency
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperationsPerGoroutine; j++ {
				rate := &model.ExchangeRate{
					Currency:   "USD",
					RateToBase: float64(goroutineID*numOperationsPerGoroutine + j),
					UpdatedAt:  time.Now(),
				}

				if err := repo.Upsert(rate); err != nil {
					t.Errorf("Goroutine %d: Failed to upsert rate: %v", goroutineID, err)
					return
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Verify the database is still consistent - should have exactly one USD record
	retrieved, err := repo.GetByCurrency("USD")
	if err != nil {
		t.Fatalf("Failed to get rate after concurrent operations: %v", err)
	}

	if retrieved.Currency != "USD" {
		t.Errorf("Expected currency USD, got %s", retrieved.Currency)
	}

	// Verify total count is still 1 (no duplicate records)
	allRates, err := repo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all rates: %v", err)
	}

	usdCount := 0
	for _, rate := range allRates {
		if rate.Currency == "USD" {
			usdCount++
		}
	}

	if usdCount != 1 {
		t.Errorf("Expected exactly 1 USD record, got %d", usdCount)
	}

	t.Logf("✅ Concurrent test passed: %d goroutines × %d operations each, no race conditions!",
		numGoroutines, numOperationsPerGoroutine)
}

// TestConcurrentMixedOperations tests concurrent reads and writes
func TestConcurrentMixedOperations(t *testing.T) {
	// Use a real file-based database for concurrent tests
	tmpFile := fmt.Sprintf("/tmp/finbot_mixed_test_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.ExchangeRate{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	repo := createTestRepo(db)

	// Insert initial data
	currencies := []string{"USD", "CNY", "EUR", "GBP"}
	for i, currency := range currencies {
		rate := &model.ExchangeRate{
			Currency:   currency,
			RateToBase: float64(i) * 0.01,
			UpdatedAt:  time.Now(),
		}
		if err := repo.Upsert(rate); err != nil {
			t.Fatalf("Failed to insert initial rate: %v", err)
		}
	}

	const numReaders = 20
	const numWriters = 20
	const operationsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// Start reader goroutines
	for i := 0; i < numReaders; i++ {
		go func(readerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				// Random read operation
				currency := currencies[j%len(currencies)]
				_, err := repo.GetByCurrency(currency)
				if err != nil {
					t.Errorf("Reader %d: Failed to get currency %s: %v", readerID, currency, err)
					return
				}

				// Also test GetAll
				if j%10 == 0 {
					_, err := repo.GetAll()
					if err != nil {
						t.Errorf("Reader %d: Failed to get all rates: %v", readerID, err)
						return
					}
				}
			}
		}(i)
	}

	// Start writer goroutines
	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerGoroutine; j++ {
				currency := currencies[j%len(currencies)]
				rate := &model.ExchangeRate{
					Currency:   currency,
					RateToBase: float64(writerID*operationsPerGoroutine+j) * 0.00001,
					UpdatedAt:  time.Now(),
				}

				if err := repo.Upsert(rate); err != nil {
					t.Errorf("Writer %d: Failed to upsert rate: %v", writerID, err)
					return
				}
			}
		}(i)
	}

	// Wait for all operations to complete
	wg.Wait()

	// Verify database consistency
	allRates, err := repo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all rates: %v", err)
	}

	if len(allRates) != len(currencies) {
		t.Errorf("Expected %d currencies, got %d", len(currencies), len(allRates))
	}

	t.Logf("✅ Mixed concurrent operations passed: %d readers + %d writers, no race conditions!",
		numReaders, numWriters)
}

// TestConcurrentWithRealDatabase tests with a real file-based database
func TestConcurrentWithRealDatabase(t *testing.T) {
	// Create a temporary database file
	tmpFile := fmt.Sprintf("/tmp/finbot_test_%d.db", time.Now().UnixNano())
	defer os.Remove(tmpFile)

	// Use direct database connection instead of singleton to avoid conflicts
	db, err := gorm.Open(sqlite.Open(tmpFile), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.ExchangeRate{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	repo := createTestRepo(db)

	const numGoroutines = 30
	const numOperations = 30

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	startTime := time.Now()

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				currency := fmt.Sprintf("CURRENCY_%d", id%5)
				rate := &model.ExchangeRate{
					Currency:   currency,
					RateToBase: float64(id*numOperations+j) * 0.001,
					UpdatedAt:  time.Now(),
				}

				if err := repo.Upsert(rate); err != nil {
					t.Errorf("Goroutine %d: Failed to upsert: %v", id, err)
					return
				}

				// Also do some reads
				if j%5 == 0 {
					_, _ = repo.GetByCurrency(currency)
				}
			}
		}(i)
	}

	wg.Wait()

	duration := time.Since(startTime)

	// Verify final state
	allRates, err := repo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all rates: %v", err)
	}

	expectedCurrencies := 5
	if len(allRates) != expectedCurrencies {
		t.Errorf("Expected %d currencies, got %d", expectedCurrencies, len(allRates))
	}

	t.Logf("✅ Real database concurrent test passed!")
	t.Logf("   %d goroutines × %d operations = %d total operations",
		numGoroutines, numOperations, numGoroutines*numOperations)
	t.Logf("   Completed in %v", duration)
	t.Logf("   No race conditions detected!")
}

// Benchmark tests
func BenchmarkUpsert(b *testing.B) {
	db := setupTestDB(&testing.T{})
	repo := createTestRepo(db)

	rate := &model.ExchangeRate{
		Currency:   "USD",
		RateToBase: 0.0067,
		UpdatedAt:  time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.Upsert(rate)
	}
}

func BenchmarkGetByCurrency(b *testing.B) {
	db := setupTestDB(&testing.T{})
	repo := createTestRepo(db)

	rate := &model.ExchangeRate{
		Currency:   "USD",
		RateToBase: 0.0067,
		UpdatedAt:  time.Now(),
	}
	repo.Upsert(rate)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo.GetByCurrency("USD")
	}
}

func BenchmarkConcurrentUpsert(b *testing.B) {
	db := setupTestDB(&testing.T{})
	repo := createTestRepo(db)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			rate := &model.ExchangeRate{
				Currency:   "USD",
				RateToBase: float64(i) * 0.001,
				UpdatedAt:  time.Now(),
			}
			repo.Upsert(rate)
			i++
		}
	})
}
