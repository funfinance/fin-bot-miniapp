package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"fin-bot-miniapp/internal/logger"
	"fin-bot-miniapp/internal/model"
	"fin-bot-miniapp/internal/repository"
)

var ErrRateNotFound = errors.New("exchange rate not found")

type RateService struct {
	repo                repository.RateRepository
	cache               map[string]*model.ExchangeRate
	cacheMu             sync.RWMutex
	updateInterval      time.Duration
	apiKey              string
	baseCurrency        string
	supportedCurrencies []string
}

type ExchangeRateAPIResponse struct {
	Result             string             `json:"result"`
	BaseCode           string             `json:"base_code"`
	ConversionRates    map[string]float64 `json:"conversion_rates"`
	TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
}

func NewRateService(repo repository.RateRepository, updateInterval time.Duration, apiKey, baseCurrency string, supportedCurrencies []string) *RateService {
	service := &RateService{
		repo:                repo,
		cache:               make(map[string]*model.ExchangeRate),
		updateInterval:      updateInterval,
		apiKey:              apiKey,
		baseCurrency:        baseCurrency,
		supportedCurrencies: supportedCurrencies,
	}

	if baseCurrency != "JPY" && apiKey == "" {
		logger.Warn("WARNING: base_currency is %q but no API key is configured. "+
			"Fallback exchange rates are JPY-based and will be incorrect. "+
			"Please set rate.api_key in config.", baseCurrency)
	}

	if err := service.loadRatesFromDB(); err != nil {
		logger.Warn("Failed to load initial rates: %v", err)
		service.initializeDefaultRates()
	}

	go service.startDailyMidnightUpdate()

	return service
}

func (s *RateService) loadRatesFromDB() error {
	rates, err := s.repo.GetAll()
	if err != nil {
		return err
	}

	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	for _, rate := range rates {
		s.cache[rate.Currency] = rate
	}

	logger.Info("Loaded %d exchange rates from database", len(rates))
	return nil
}

func (s *RateService) initializeDefaultRates() {
	defaultRates := map[string]float64{
		"USD": 0.0067,
		"EUR": 0.0062,
		"GBP": 0.0053,
		"CHF": 0.0060,
		"CAD": 0.0091,
		"AUD": 0.0098,
		"CNY": 0.048,
		"KRW": 9.2,
		"HKD": 0.052,
		"TWD": 0.21,
		"SGD": 0.0089,
		"MYR": 0.030,
		"THB": 0.23,
		"VND": 168.0,
		"INR": 0.56,
	}

	for currency, rate := range defaultRates {
		if err := s.UpdateRate(currency, rate); err != nil {
			logger.Error("Failed to initialize rate for %s: %v", currency, err)
		}
	}

	logger.Info("Initialized default exchange rates")
}

func (s *RateService) BaseCurrency() string {
	return s.baseCurrency
}

func (s *RateService) GetRate(currency string) (float64, error) {
	if currency == s.baseCurrency {
		return 1.0, nil
	}

	s.cacheMu.RLock()
	rate, exists := s.cache[currency]
	s.cacheMu.RUnlock()

	if exists {
		return rate.RateToBase, nil
	}

	rate, err := s.repo.GetByCurrency(currency)
	if err != nil {
		return 0, ErrRateNotFound
	}

	s.cacheMu.Lock()
	s.cache[currency] = rate
	s.cacheMu.Unlock()

	return rate.RateToBase, nil
}

func (s *RateService) ConvertToBase(amount float64, currency string) (float64, error) {
	rate, err := s.GetRate(currency)
	if err != nil {
		return 0, err
	}

	if rate == 0 {
		return 0, fmt.Errorf("invalid exchange rate: %f", rate)
	}

	return amount / rate, nil
}

func (s *RateService) UpdateRate(currency string, rateToBase float64) error {
	rate := &model.ExchangeRate{
		Currency:   currency,
		RateToBase: rateToBase,
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Upsert(rate); err != nil {
		logger.Error("Failed to update rate for %s: %v", currency, err)
		return fmt.Errorf("update rate: %w", err)
	}

	s.cacheMu.Lock()
	s.cache[currency] = rate
	s.cacheMu.Unlock()

	logger.Info("Updated exchange rate: 1 %s = %.6f %s", s.baseCurrency, rateToBase, currency)
	return nil
}

func (s *RateService) startDailyMidnightUpdate() {
	if s.apiKey == "" {
		logger.Warn("Exchange rate API key not configured, skipping automatic updates")
		logger.Info("Please register at https://www.exchangerate-api.com/ and add API key to config.yaml")
		return
	}

	if err := s.updateRatesFromAPI(); err != nil {
		logger.Error("Failed initial rate update from API: %v", err)
	}

	for {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		duration := nextMidnight.Sub(now)

		logger.Info("Next rate update scheduled at %s (in %s)", nextMidnight.Format("2006-01-02 15:04:05"), duration)

		time.Sleep(duration)

		if err := s.updateRatesFromAPI(); err != nil {
			logger.Error("Failed to update rates from API at midnight: %v", err)
		}
	}
}

func (s *RateService) GetSupportedCurrencies() []string {
	return s.supportedCurrencies
}

func (s *RateService) updateRatesFromAPI() error {
	if s.apiKey == "" {
		return fmt.Errorf("API key not configured")
	}

	apiURL := fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/latest/%s", s.apiKey, s.baseCurrency)

	logger.Info("Fetching exchange rates from API...")

	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp ExchangeRateAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Result != "success" {
		return fmt.Errorf("API returned result: %s", apiResp.Result)
	}

	updatedCount := 0
	for _, currency := range s.supportedCurrencies {
		if rate, exists := apiResp.ConversionRates[currency]; exists {
			if err := s.UpdateRate(currency, rate); err != nil {
				logger.Error("Failed to update %s rate: %v", currency, err)
			} else {
				updatedCount++
			}
		}
	}

	logger.Info("Successfully updated %d exchange rates from API", updatedCount)
	return nil
}
