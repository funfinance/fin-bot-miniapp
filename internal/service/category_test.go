package service

import (
	"testing"

	"fin-bot-miniapp/internal/model"

	"gorm.io/gorm"
)

// mockCategoryRepo is used by category service tests
type mockCategoryRepo struct {
	categories map[int64]map[string]*model.Category
}

func newMockCategoryRepo() *mockCategoryRepo {
	return &mockCategoryRepo{
		categories: make(map[int64]map[string]*model.Category),
	}
}

func (m *mockCategoryRepo) Upsert(category *model.Category) error {
	if m.categories[category.UserID] == nil {
		m.categories[category.UserID] = make(map[string]*model.Category)
	}
	m.categories[category.UserID][category.Code] = category
	return nil
}

func (m *mockCategoryRepo) GetByUserIDAndCode(userID int64, code string) (*model.Category, error) {
	userCats, exists := m.categories[userID]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	cat, exists := userCats[code]
	if !exists {
		return nil, gorm.ErrRecordNotFound
	}
	return cat, nil
}

func (m *mockCategoryRepo) GetActiveByUserID(userID int64) ([]*model.Category, error) {
	userCats, exists := m.categories[userID]
	if !exists {
		return []*model.Category{}, nil
	}
	var categories []*model.Category
	for _, cat := range userCats {
		if cat.Active {
			categories = append(categories, cat)
		}
	}
	return categories, nil
}

func (m *mockCategoryRepo) GetByUserID(userID int64) ([]*model.Category, error) {
	userCats, exists := m.categories[userID]
	if !exists {
		return []*model.Category{}, nil
	}
	var categories []*model.Category
	for _, cat := range userCats {
		categories = append(categories, cat)
	}
	return categories, nil
}

func TestCategoryNewService(t *testing.T) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)

	if service == nil {
		t.Fatal("NewCategoryService should not return nil")
	}

	userID := int64(12345)
	categories, err := service.GetActiveByUserID(userID)
	if err != nil {
		t.Fatalf("Failed to get categories: %v", err)
	}

	if len(categories) != 8 {
		t.Errorf("Expected 8 default categories, got %d", len(categories))
	}

	expectedCodes := []string{"food", "shopping", "transport", "housing", "entertainment", "medical", "education", "other"}
	for _, code := range expectedCodes {
		found := false
		for _, cat := range categories {
			if cat.Code == code {
				found = true
				if !cat.Active {
					t.Errorf("Category %s should be active", code)
				}
				break
			}
		}
		if !found {
			t.Errorf("Expected category %s not found", code)
		}
	}
}

func TestCategoryGetAllActive(t *testing.T) {
	repo := newMockCategoryRepo()
	userID := int64(12345)

	repo.Upsert(&model.Category{UserID: userID, Code: "food", Name: "Food", Emoji: "🍔", Active: true})
	repo.Upsert(&model.Category{UserID: userID, Code: "shopping", Name: "Shopping", Emoji: "🛒", Active: true})
	repo.Upsert(&model.Category{UserID: userID, Code: "inactive", Name: "Inactive", Emoji: "❌", Active: false})

	service := NewCategoryService(repo)

	categories, err := service.GetActiveByUserID(userID)
	if err != nil {
		t.Fatalf("Failed to get categories: %v", err)
	}

	if len(categories) != 2 {
		t.Errorf("Expected 2 active categories, got %d", len(categories))
	}

	for _, cat := range categories {
		if !cat.Active {
			t.Errorf("Category %s should be active", cat.Code)
		}
		if cat.Code == "inactive" {
			t.Errorf("Inactive category should not be returned")
		}
	}
}

func TestCategoryGetByCode(t *testing.T) {
	repo := newMockCategoryRepo()
	userID := int64(12345)

	repo.Upsert(&model.Category{
		UserID: userID,
		Code:   "food",
		Name:   "Food",
		Emoji:  "🍔",
		Active: true,
	})

	service := NewCategoryService(repo)

	cat, err := service.GetByUserIDAndCode(userID, "food")
	if err != nil {
		t.Fatalf("Failed to get category: %v", err)
	}

	if cat.Code != "food" {
		t.Errorf("Expected code 'food', got '%s'", cat.Code)
	}

	if cat.Name != "Food" {
		t.Errorf("Expected name 'Food', got '%s'", cat.Name)
	}

	if cat.Emoji != "🍔" {
		t.Errorf("Expected emoji '🍔', got '%s'", cat.Emoji)
	}

	_, err = service.GetByUserIDAndCode(userID, "nonexistent")
	if err != ErrCategoryNotFound {
		t.Errorf("Expected ErrCategoryNotFound, got %v", err)
	}
}

func TestCategoryCacheIsolation(t *testing.T) {
	repo := newMockCategoryRepo()
	userID := int64(12345)

	repo.Upsert(&model.Category{UserID: userID, Code: "food", Name: "Food", Emoji: "🍔", Active: true})
	repo.Upsert(&model.Category{UserID: userID, Code: "shopping", Name: "Shopping", Emoji: "🛒", Active: true})

	service := NewCategoryService(repo)

	categories, _ := service.GetActiveByUserID(userID)

	originalLen := len(categories)
	categories = append(categories, &model.Category{UserID: userID, Code: "test", Name: "Test", Emoji: "✅", Active: true})

	categories2, _ := service.GetActiveByUserID(userID)

	if len(categories2) != originalLen {
		t.Errorf("Expected %d categories in cache, got %d", originalLen, len(categories2))
	}

	for _, cat := range categories2 {
		if cat.Code == "test" {
			t.Error("Cache should not be affected by external slice modification")
		}
	}
}


func TestCategoryDefaultCategoriesOrder(t *testing.T) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	categories, _ := service.GetActiveByUserID(userID)

	expectedOrder := []string{"food", "shopping", "transport", "housing", "entertainment", "medical", "education", "other"}

	if len(categories) != len(expectedOrder) {
		t.Fatalf("Expected %d categories, got %d", len(expectedOrder), len(categories))
	}

	for i, expected := range expectedOrder {
		found := false
		for _, cat := range categories {
			if cat.Code == expected {
				if cat.SortOrder != i+1 {
					t.Errorf("Category %s: expected sort order %d, got %d", expected, i+1, cat.SortOrder)
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Category %s not found", expected)
		}
	}
}

func TestCategoryDefaultCategoriesEmojis(t *testing.T) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	service.GetActiveByUserID(userID)

	expectedEmojis := map[string]string{
		"food":          "🍔",
		"shopping":      "🛒",
		"transport":     "🚇",
		"housing":       "🏠",
		"entertainment": "🎬",
		"medical":       "🏥",
		"education":     "📚",
		"other":         "💰",
	}

	for code, expectedEmoji := range expectedEmojis {
		cat, err := service.GetByUserIDAndCode(userID, code)
		if err != nil {
			t.Errorf("Category %s not found: %v", code, err)
			continue
		}

		if cat.Emoji != expectedEmoji {
			t.Errorf("Category %s: expected emoji '%s', got '%s'", code, expectedEmoji, cat.Emoji)
		}
	}
}

func TestCategoryAddCategory(t *testing.T) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	initialCategories, _ := service.GetActiveByUserID(userID)
	initialCount := len(initialCategories)

	err := service.AddCategory(userID, "gym", "Gym", "🏋️", 10)
	if err != nil {
		t.Fatalf("Failed to add category: %v", err)
	}

	cat, err := repo.GetByUserIDAndCode(userID, "gym")
	if err != nil {
		t.Fatalf("Category not found in repository: %v", err)
	}

	if cat.Code != "gym" {
		t.Errorf("Expected code 'gym', got '%s'", cat.Code)
	}
	if cat.Name != "Gym" {
		t.Errorf("Expected name 'Gym', got '%s'", cat.Name)
	}
	if cat.Emoji != "🏋️" {
		t.Errorf("Expected emoji '🏋️', got '%s'", cat.Emoji)
	}
	if cat.SortOrder != 10 {
		t.Errorf("Expected sort order 10, got %d", cat.SortOrder)
	}
	if !cat.Active {
		t.Error("Expected category to be active")
	}

	categories, _ := service.GetActiveByUserID(userID)
	if len(categories) != initialCount+1 {
		t.Errorf("Expected %d categories after add, got %d", initialCount+1, len(categories))
	}

	found := false
	for _, c := range categories {
		if c.Code == "gym" {
			found = true
			break
		}
	}
	if !found {
		t.Error("New category not found in cache after add")
	}

	cachedCat, err := service.GetByUserIDAndCode(userID, "gym")
	if err != nil {
		t.Fatalf("Failed to get newly added category: %v", err)
	}
	if cachedCat.Code != "gym" {
		t.Errorf("Expected code 'gym', got '%s'", cachedCat.Code)
	}
}

func TestCategoryAddCategoryDuplicate(t *testing.T) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	service.GetActiveByUserID(userID)

	err := service.AddCategory(userID, "pets", "Pets", "🐶", 20)
	if err != nil {
		t.Fatalf("Failed to add category first time: %v", err)
	}

	cat, err := repo.GetByUserIDAndCode(userID, "pets")
	if err != nil {
		t.Fatalf("Category not found in repo after add: %v", err)
	}
	if cat.Name != "Pets" {
		t.Errorf("Expected name 'Pets', got '%s'", cat.Name)
	}

	err = service.AddCategory(userID, "pets", "Pets & Animals", "🐱", 21)
	if err != ErrCategoryCodeExists {
		debugCat, debugErr := repo.GetByUserIDAndCode(userID, "pets")
		t.Errorf("Expected ErrCategoryCodeExists when adding duplicate, got %v. Repo state: cat=%+v, err=%v", err, debugCat, debugErr)
	}

	cat, err = repo.GetByUserIDAndCode(userID, "pets")
	if err != nil {
		t.Fatalf("Category not found: %v", err)
	}
	if cat.Name != "Pets" {
		t.Errorf("Expected name 'Pets' (unchanged), got '%s'", cat.Name)
	}
}

func TestCategoryConcurrentAccess(t *testing.T) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	service.GetActiveByUserID(userID)

	done := make(chan bool, 20)

	for i := 0; i < 20; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				service.GetActiveByUserID(userID)
				service.GetByUserIDAndCode(userID, "food")
			}
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	categories, _ := service.GetActiveByUserID(userID)
	if len(categories) == 0 {
		t.Error("Service broken after concurrent access")
	}
}

func BenchmarkCategoryGetAllActive(b *testing.B) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	service.GetActiveByUserID(userID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetActiveByUserID(userID)
	}
}

func BenchmarkCategoryGetByCode(b *testing.B) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	service.GetActiveByUserID(userID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetByUserIDAndCode(userID, "food")
	}
}

func BenchmarkCategoryConcurrentGetAllActive(b *testing.B) {
	repo := newMockCategoryRepo()
	service := NewCategoryService(repo)
	userID := int64(12345)

	service.GetActiveByUserID(userID)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.GetActiveByUserID(userID)
		}
	})
}
