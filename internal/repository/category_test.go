package repository

import (
	"testing"

	"fin-bot-miniapp/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupCategoryTestDB creates a test database in memory
func setupCategoryTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&model.Category{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func createCategoryTestRepo(db *gorm.DB) CategoryRepository {
	return &categoryRepo{db: db}
}

func TestCategoryUpsert(t *testing.T) {
	db := setupCategoryTestDB(t)
	repo := createCategoryTestRepo(db)
	userID := int64(12345)

	// Test inserting a new category
	category := &model.Category{
		UserID:    userID,
		Code:      "food",
		Name:      "Food",
		Emoji:     "🍔",
		SortOrder: 1,
		Active:    true,
	}

	err := repo.Upsert(category)
	if err != nil {
		t.Fatalf("Failed to insert category: %v", err)
	}

	// Verify the category was inserted
	retrieved, err := repo.GetByUserIDAndCode(userID, "food")
	if err != nil {
		t.Fatalf("Failed to get category: %v", err)
	}

	if retrieved.Code != "food" {
		t.Errorf("Expected code 'food', got '%s'", retrieved.Code)
	}

	if retrieved.Name != "Food" {
		t.Errorf("Expected name 'Food', got '%s'", retrieved.Name)
	}

	// Test updating existing category
	category.Name = "Food & Drinks"
	category.Emoji = "🍽️"
	err = repo.Upsert(category)
	if err != nil {
		t.Fatalf("Failed to update category: %v", err)
	}

	// Verify the category was updated
	retrieved, err = repo.GetByUserIDAndCode(userID, "food")
	if err != nil {
		t.Fatalf("Failed to get updated category: %v", err)
	}

	if retrieved.Name != "Food & Drinks" {
		t.Errorf("Expected updated name 'Food & Drinks', got '%s'", retrieved.Name)
	}

	if retrieved.Emoji != "🍽️" {
		t.Errorf("Expected updated emoji '🍽️', got '%s'", retrieved.Emoji)
	}
}

func TestCategoryGetByUserIDAndCode(t *testing.T) {
	db := setupCategoryTestDB(t)
	repo := createCategoryTestRepo(db)
	userID := int64(12345)

	// Insert test data
	category := &model.Category{
		UserID:    userID,
		Code:      "transport",
		Name:      "Transport",
		Emoji:     "🚇",
		SortOrder: 3,
		Active:    true,
	}
	repo.Upsert(category)

	// Test getting existing category
	retrieved, err := repo.GetByUserIDAndCode(userID, "transport")
	if err != nil {
		t.Fatalf("Failed to get category: %v", err)
	}

	if retrieved.Code != "transport" {
		t.Errorf("Expected code 'transport', got '%s'", retrieved.Code)
	}

	// Test getting non-existent category
	_, err = repo.GetByUserIDAndCode(userID, "nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent category, got nil")
	}

	// Test getting category for different user
	_, err = repo.GetByUserIDAndCode(67890, "transport")
	if err == nil {
		t.Error("Expected error for category from different user, got nil")
	}
}

func TestCategoryGetByUserID(t *testing.T) {
	db := setupCategoryTestDB(t)
	repo := createCategoryTestRepo(db)
	userID1 := int64(12345)
	userID2 := int64(67890)

	// Insert multiple categories for different users
	categories := []*model.Category{
		{UserID: userID1, Code: "food", Name: "Food", Emoji: "🍔", SortOrder: 1, Active: true},
		{UserID: userID1, Code: "shopping", Name: "Shopping", Emoji: "🛒", SortOrder: 2, Active: true},
		{UserID: userID1, Code: "transport", Name: "Transport", Emoji: "🚇", SortOrder: 3, Active: false},
		{UserID: userID2, Code: "food", Name: "Food", Emoji: "🍔", SortOrder: 1, Active: true},
	}

	for _, cat := range categories {
		if err := repo.Upsert(cat); err != nil {
			t.Fatalf("Failed to insert category: %v", err)
		}
	}

	// Get all categories for user1
	user1Categories, err := repo.GetByUserID(userID1)
	if err != nil {
		t.Fatalf("Failed to get categories for user1: %v", err)
	}

	if len(user1Categories) != 3 {
		t.Errorf("Expected 3 categories for user1, got %d", len(user1Categories))
	}

	// Get all categories for user2
	user2Categories, err := repo.GetByUserID(userID2)
	if err != nil {
		t.Fatalf("Failed to get categories for user2: %v", err)
	}

	if len(user2Categories) != 1 {
		t.Errorf("Expected 1 category for user2, got %d", len(user2Categories))
	}
}

func TestCategoryGetActiveByUserID(t *testing.T) {
	db := setupCategoryTestDB(t)
	repo := createCategoryTestRepo(db)
	userID := int64(12345)

	// Insert multiple categories
	categories := []*model.Category{
		{UserID: userID, Code: "food", Name: "Food", Emoji: "🍔", SortOrder: 1, Active: true},
		{UserID: userID, Code: "shopping", Name: "Shopping", Emoji: "🛒", SortOrder: 2, Active: true},
		{UserID: userID, Code: "transport", Name: "Transport", Emoji: "🚇", SortOrder: 3, Active: true}, // Will be set to false below
	}

	for _, cat := range categories {
		if err := repo.Upsert(cat); err != nil {
			t.Fatalf("Failed to insert category: %v", err)
		}
	}

	// GORM's default:true tag overrides explicit false, so manually update inactive category
	db.Model(&model.Category{}).Where("user_id = ? AND code = ?", userID, "transport").Update("active", false)

	// Get only active categories
	activeCategories, err := repo.GetActiveByUserID(userID)
	if err != nil {
		t.Fatalf("Failed to get active categories: %v", err)
	}

	if len(activeCategories) != 2 {
		t.Errorf("Expected 2 active categories, got %d", len(activeCategories))
	}

	// Verify all returned categories are active
	for _, cat := range activeCategories {
		if !cat.Active {
			t.Errorf("Category %s should be active", cat.Code)
		}
	}
}

func TestCategoryUpsertUserIsolation(t *testing.T) {
	db := setupCategoryTestDB(t)
	repo := createCategoryTestRepo(db)
	userID1 := int64(12345)
	userID2 := int64(67890)

	// User1 creates a category
	category1 := &model.Category{
		UserID:    userID1,
		Code:      "food",
		Name:      "Food User 1",
		Emoji:     "🍔",
		SortOrder: 1,
		Active:    true,
	}
	repo.Upsert(category1)

	// User2 creates a category with the same code (should work - different users)
	category2 := &model.Category{
		UserID:    userID2,
		Code:      "food",
		Name:      "Food User 2",
		Emoji:     "🍕",
		SortOrder: 1,
		Active:    true,
	}
	err := repo.Upsert(category2)
	if err != nil {
		t.Fatalf("User2 should be able to create category with same code as User1: %v", err)
	}

	// Verify both categories exist and are isolated
	cat1, err := repo.GetByUserIDAndCode(userID1, "food")
	if err != nil {
		t.Fatalf("Failed to get User1's category: %v", err)
	}
	if cat1.Name != "Food User 1" {
		t.Errorf("Expected 'Food User 1', got '%s'", cat1.Name)
	}

	cat2, err := repo.GetByUserIDAndCode(userID2, "food")
	if err != nil {
		t.Fatalf("Failed to get User2's category: %v", err)
	}
	if cat2.Name != "Food User 2" {
		t.Errorf("Expected 'Food User 2', got '%s'", cat2.Name)
	}
}
