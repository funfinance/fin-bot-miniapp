package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fin-bot-miniapp/internal/api"
	"fin-bot-miniapp/internal/bot"
	"fin-bot-miniapp/internal/config"
	"fin-bot-miniapp/internal/database"
	"fin-bot-miniapp/internal/logger"
	"fin-bot-miniapp/internal/model"
	"fin-bot-miniapp/internal/repository"
	"fin-bot-miniapp/internal/service"
	"fin-bot-miniapp/webfs"

	tele "gopkg.in/telebot.v3"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.Logger.Level, cfg.Logger.File); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	logger.Info("=== Finance Bot (Mini App) Starting ===")

	if err := database.Init(cfg.Database.Path); err != nil {
		logger.Error("Failed to init database: %v", err)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			logger.Error("Failed to close database: %v", err)
		}
	}()

	db := database.Get()
	if err := db.AutoMigrate(&model.Expense{}, &model.ExchangeRate{}, &model.Category{}, &model.Ledger{}, &model.RecurringExpense{}); err != nil {
		logger.Error("Failed to migrate database: %v", err)
		os.Exit(1)
	}

	botInstance, err := tele.NewBot(tele.Settings{
		Token:  cfg.Bot.Token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		logger.Error("Failed to create bot: %v", err)
		os.Exit(1)
	}

	expenseRepo := repository.NewExpenseRepository()
	rateRepo := repository.NewRateRepository()
	categoryRepo := repository.NewCategoryRepository()
	ledgerRepo := repository.NewLedgerRepository()

	rateService := service.NewRateService(rateRepo, cfg.Rate.UpdateInterval, cfg.Rate.APIKey, cfg.Rate.BaseCurrency, cfg.Rate.SupportedCurrencies)
	categoryService := service.NewCategoryService(categoryRepo)
	ledgerService := service.NewLedgerService(ledgerRepo)
	expenseService := service.NewExpenseService(expenseRepo, rateService)
	recurringRepo := repository.NewRecurringExpenseRepository()
	recurringService := service.NewRecurringService(recurringRepo, expenseRepo, rateService)

	handler := bot.NewHandler(botInstance, expenseService, categoryService, ledgerService, rateService, cfg.Server.MiniAppURL)
	handler.Register()

	apiServer := api.NewServer(expenseService, categoryService, ledgerService, rateService, recurringService, cfg.Bot.Token, cfg.Server, webfs.FS(), handler)
	go func() {
		logger.Info("API server starting on %s", cfg.Server.Addr)
		if err := apiServer.Start(); err != nil {
			logger.Error("API server error: %v", err)
		}
	}()

	schedulerCtx, schedulerCancel := context.WithCancel(context.Background())
	go recurringService.Start(schedulerCtx)

	go func() {
		logger.Info("Bot started. Waiting for messages...")
		botInstance.Start()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down...")
	schedulerCancel()
	botInstance.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := apiServer.Shutdown(ctx); err != nil {
		logger.Error("API server shutdown error: %v", err)
	}

	logger.Info("=== Finance Bot Stopped ===")
	logger.Close()
}
