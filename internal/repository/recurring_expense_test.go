package repository

import (
	"testing"
	"time"

	"fin-bot-miniapp/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRecurringTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.RecurringExpense{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func createRecurringTestRepo(db *gorm.DB) RecurringExpenseRepository {
	return &recurringExpenseRepo{db: db}
}

func TestRecurringRepository_Create(t *testing.T) {
	db := setupRecurringTestDB(t)
	repo := createRecurringTestRepo(db)

	next := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	r := &model.RecurringExpense{
		UserID:        12345,
		Username:      "testuser",
		LedgerID:      1,
		Type:          "expense",
		Amount:        100,
		Currency:      "JPY",
		Category:      "food",
		Frequency:     "monthly",
		Days:          "1,15",
		NextTriggerAt: next,
		Active:        true,
	}

	if err := repo.Create(r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID == 0 {
		t.Error("expected ID to be set after create")
	}
}

func TestRecurringRepository_GetDue(t *testing.T) {
	db := setupRecurringTestDB(t)
	repo := createRecurringTestRepo(db)

	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(24 * time.Hour)

	due := &model.RecurringExpense{UserID: 1, Frequency: "monthly", Days: "1", Amount: 100, Currency: "JPY", NextTriggerAt: past, Active: true}
	notYet := &model.RecurringExpense{UserID: 1, Frequency: "monthly", Days: "1", Amount: 200, Currency: "JPY", NextTriggerAt: future, Active: true}
	inactive := &model.RecurringExpense{UserID: 1, Frequency: "monthly", Days: "1", Amount: 300, Currency: "JPY", NextTriggerAt: past, Active: false}

	repo.Create(due)
	repo.Create(notYet)
	repo.Create(inactive)
	// GORM default:true override — explicitly set inactive
	db.Model(&model.RecurringExpense{}).Where("id = ?", inactive.ID).Update("active", false)

	results, err := repo.GetDue(now)
	if err != nil {
		t.Fatalf("GetDue: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 due record, got %d", len(results))
	}
	if results[0].ID != due.ID {
		t.Errorf("expected due record id=%d, got id=%d", due.ID, results[0].ID)
	}
}

func TestRecurringRepository_UpdateTrigger(t *testing.T) {
	db := setupRecurringTestDB(t)
	repo := createRecurringTestRepo(db)

	next := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	r := &model.RecurringExpense{UserID: 1, Frequency: "monthly", Days: "1", Amount: 100, Currency: "JPY", NextTriggerAt: next, Active: true}
	repo.Create(r)

	triggered := time.Now().Truncate(time.Second)
	newNext := next.AddDate(0, 1, 0).Truncate(time.Second)

	if err := repo.UpdateTrigger(r.ID, triggered, newNext); err != nil {
		t.Fatalf("UpdateTrigger: %v", err)
	}

	var updated model.RecurringExpense
	db.First(&updated, r.ID)

	if updated.LastTriggeredAt == nil {
		t.Fatal("expected LastTriggeredAt to be set")
	}
	if !updated.LastTriggeredAt.Truncate(time.Second).Equal(triggered) {
		t.Errorf("LastTriggeredAt: got %v, want %v", updated.LastTriggeredAt, triggered)
	}
	if !updated.NextTriggerAt.Truncate(time.Second).Equal(newNext) {
		t.Errorf("NextTriggerAt: got %v, want %v", updated.NextTriggerAt, newNext)
	}
}

func TestRecurringRepository_Deactivate(t *testing.T) {
	db := setupRecurringTestDB(t)
	repo := createRecurringTestRepo(db)

	r := &model.RecurringExpense{UserID: 12345, Frequency: "monthly", Days: "1", Amount: 100, Currency: "JPY", NextTriggerAt: time.Now(), Active: true}
	repo.Create(r)

	if err := repo.Deactivate(r.ID, 12345); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	var updated model.RecurringExpense
	db.First(&updated, r.ID)
	if updated.Active {
		t.Error("expected Active to be false after deactivate")
	}
}

func TestRecurringRepository_DeactivateWrongUser(t *testing.T) {
	db := setupRecurringTestDB(t)
	repo := createRecurringTestRepo(db)

	r := &model.RecurringExpense{UserID: 12345, Frequency: "monthly", Days: "1", Amount: 100, Currency: "JPY", NextTriggerAt: time.Now(), Active: true}
	repo.Create(r)

	// different user — should not deactivate
	repo.Deactivate(r.ID, 99999)

	var updated model.RecurringExpense
	db.First(&updated, r.ID)
	if !updated.Active {
		t.Error("record should still be active when wrong user tries to deactivate")
	}
}
