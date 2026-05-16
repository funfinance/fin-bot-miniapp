package service

import (
	"sync"
	"testing"
	"time"

	"fin-bot-miniapp/internal/model"
)

// mockRateRepo is used by rate service tests
type mockRateRepo struct {
	rates map[string]*model.ExchangeRate
	mu    sync.RWMutex
}

func newMockRateRepo() *mockRateRepo {
	return &mockRateRepo{
		rates: make(map[string]*model.ExchangeRate),
	}
}

func (m *mockRateRepo) Upsert(rate *model.ExchangeRate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rates[rate.Currency] = rate
	return nil
}

func (m *mockRateRepo) GetByCurrency(currency string) (*model.ExchangeRate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rate, exists := m.rates[currency]
	if !exists {
		return nil, ErrRateNotFound
	}
	return rate, nil
}

func (m *mockRateRepo) GetAll() ([]*model.ExchangeRate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var rates []*model.ExchangeRate
	for _, rate := range m.rates {
		rates = append(rates, rate)
	}
	return rates, nil
}

func TestUpdateRate(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD", "CNY"})

	err := service.UpdateRate("USD", 0.0067)
	if err != nil {
		t.Fatalf("Failed to update rate: %v", err)
	}

	rate, err := service.GetRate("USD")
	if err != nil {
		t.Fatalf("Failed to get rate: %v", err)
	}

	if rate != 0.0067 {
		t.Errorf("Expected rate 0.0067, got %f", rate)
	}

	storedRate, err := repo.GetByCurrency("USD")
	if err != nil {
		t.Fatalf("Rate not found in repo: %v", err)
	}

	if storedRate.RateToBase != 0.0067 {
		t.Errorf("Expected stored rate 0.0067, got %f", storedRate.RateToBase)
	}
}

func TestGetRate(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD", "CNY"})

	service.UpdateRate("USD", 0.0067)

	rate, err := service.GetRate("USD")
	if err != nil {
		t.Fatalf("Failed to get rate: %v", err)
	}

	if rate != 0.0067 {
		t.Errorf("Expected rate 0.0067, got %f", rate)
	}

	_, err = service.GetRate("EUR")
	if err != ErrRateNotFound {
		t.Errorf("Expected ErrRateNotFound, got %v", err)
	}
}

func TestConvertToBase(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD", "CNY"})

	service.UpdateRate("USD", 0.0067)
	service.UpdateRate("CNY", 0.048)

	tests := []struct {
		amount   float64
		currency string
		expected float64
	}{
		{100, "USD", 14925.37},
		{50, "USD", 7462.69},
		{100, "CNY", 2083.33},
		{21, "CNY", 437.50},
	}

	for _, tt := range tests {
		result, err := service.ConvertToBase(tt.amount, tt.currency)
		if err != nil {
			t.Errorf("Failed to convert %f %s: %v", tt.amount, tt.currency, err)
			continue
		}

		diff := result - tt.expected
		if diff < 0 {
			diff = -diff
		}

		if diff > 0.01 {
			t.Errorf("Converting %f %s: expected %.2f JPY, got %.2f JPY",
				tt.amount, tt.currency, tt.expected, result)
		}
	}

	_, err := service.ConvertToBase(100, "EUR")
	if err == nil {
		t.Error("Expected error for unknown currency, got nil")
	}
}

func TestConvertToBaseWithZeroRate(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD"})

	service.UpdateRate("USD", 0)

	_, err := service.ConvertToBase(100, "USD")
	if err == nil {
		t.Error("Expected error for zero rate, got nil")
	}
}

func TestRateCacheInvalidation(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD"})

	service.UpdateRate("USD", 0.0067)

	rate1, _ := service.GetRate("USD")

	service.UpdateRate("USD", 0.0070)

	rate2, _ := service.GetRate("USD")

	if rate1 == rate2 {
		t.Error("Cache should have been updated with new rate")
	}

	if rate2 != 0.0070 {
		t.Errorf("Expected updated rate 0.0070, got %f", rate2)
	}
}

func TestGetSupportedCurrencies(t *testing.T) {
	repo := newMockRateRepo()
	expected := []string{"USD", "CNY", "EUR", "GBP"}
	service := NewRateService(repo, 24*time.Hour, "", "JPY", expected)

	currencies := service.GetSupportedCurrencies()

	if len(currencies) != len(expected) {
		t.Errorf("Expected %d currencies, got %d", len(expected), len(currencies))
	}

	for i, curr := range currencies {
		if curr != expected[i] {
			t.Errorf("Expected currency %s at index %d, got %s", expected[i], i, curr)
		}
	}
}

func TestRateConcurrentAccess(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD", "CNY"})

	service.UpdateRate("USD", 0.0067)
	service.UpdateRate("CNY", 0.048)

	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				service.GetRate("USD")
				service.ConvertToBase(100, "CNY")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				rate := 0.0067 + float64(id)*0.0001
				service.UpdateRate("USD", rate)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	rate, err := service.GetRate("USD")
	if err != nil {
		t.Errorf("Service broken after concurrent access: %v", err)
	}

	if rate <= 0 {
		t.Errorf("Invalid rate after concurrent access: %f", rate)
	}
}

func TestGetRateBaseCurrency(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD"})

	rate, err := service.GetRate("JPY")
	if err != nil {
		t.Fatalf("Unexpected error getting base currency rate: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("Expected 1.0 for base currency, got %f", rate)
	}
}

func TestGetRateCacheMissFallbackToDB(t *testing.T) {
	repo := newMockRateRepo()
	repo.rates["EUR"] = &model.ExchangeRate{Currency: "EUR", RateToBase: 0.0062}

	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"EUR"})
	service.cacheMu.Lock()
	delete(service.cache, "EUR")
	service.cacheMu.Unlock()

	rate, err := service.GetRate("EUR")
	if err != nil {
		t.Fatalf("Expected fallback to DB, got error: %v", err)
	}
	if rate != 0.0062 {
		t.Errorf("Expected rate 0.0062 from DB, got %f", rate)
	}

	rate2, err := service.GetRate("EUR")
	if err != nil || rate2 != 0.0062 {
		t.Errorf("Expected cached rate 0.0062, got %f err=%v", rate2, err)
	}
}

func TestNewRateServiceNonJPYBaseNoAPIKeyWarning(t *testing.T) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "USD", []string{"JPY", "EUR"})

	if service.BaseCurrency() != "USD" {
		t.Errorf("Expected base currency 'USD', got '%s'", service.BaseCurrency())
	}
}

func BenchmarkUpdateRate(b *testing.B) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.UpdateRate("USD", 0.0067)
	}
}

func BenchmarkGetRate(b *testing.B) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD"})
	service.UpdateRate("USD", 0.0067)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetRate("USD")
	}
}

func BenchmarkConvertToBase(b *testing.B) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD"})
	service.UpdateRate("USD", 0.0067)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ConvertToBase(100, "USD")
	}
}

func BenchmarkConcurrentGetRate(b *testing.B) {
	repo := newMockRateRepo()
	service := NewRateService(repo, 24*time.Hour, "", "JPY", []string{"USD", "CNY"})
	service.UpdateRate("USD", 0.0067)
	service.UpdateRate("CNY", 0.048)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.GetRate("USD")
		}
	})
}
