package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fin-bot-miniapp/internal/constant"
	"fin-bot-miniapp/internal/logger"
	"fin-bot-miniapp/internal/model"
	"fin-bot-miniapp/internal/service"

	"github.com/360EntSecGroup-Skylar/excelize"
)

// Notifier sends messages and files to a Telegram user.
type Notifier interface {
	NotifyUser(userID int64, msg string)
	SendFileToUser(userID int64, filename string, data []byte, caption string)
}

type handlers struct {
	expenseService   *service.ExpenseService
	categoryService  *service.CategoryService
	ledgerService    *service.LedgerService
	rateService      *service.RateService
	recurringService *service.RecurringService
	notifier         Notifier
}

// GET /api/currencies
func (h *handlers) GetCurrencies(w http.ResponseWriter, r *http.Request) {
	base := h.rateService.BaseCurrency()
	supported := h.rateService.GetSupportedCurrencies()

	seen := make(map[string]bool)
	result := []string{base}
	seen[base] = true
	for _, c := range supported {
		if !seen[c] {
			result = append(result, c)
			seen[c] = true
		}
	}
	jsonOK(w, result)
}

// GET /api/categories
func (h *handlers) GetCategories(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	cats, err := h.categoryService.GetActiveByUserID(userID)
	if err != nil {
		logger.Error("GetCategories user=%d: %v", userID, err)
		jsonError(w, "failed to get categories", http.StatusInternalServerError)
		return
	}
	resp := make([]CategoryResponse, 0, len(cats))
	for _, c := range cats {
		resp = append(resp, CategoryResponse{Code: c.Code, Name: c.Name, Emoji: c.Emoji})
	}
	jsonOK(w, resp)
}

// GET /api/ledgers
func (h *handlers) GetLedgers(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	ledgers, err := h.ledgerService.GetActiveByUserID(userID)
	if err != nil {
		logger.Error("GetLedgers user=%d: %v", userID, err)
		jsonError(w, "failed to get ledgers", http.StatusInternalServerError)
		return
	}
	resp := make([]LedgerResponse, 0, len(ledgers))
	for _, l := range ledgers {
		resp = append(resp, LedgerResponse{ID: l.ID, Code: l.Code, Name: l.Name, Emoji: l.Emoji})
	}
	jsonOK(w, resp)
}

// POST /api/expenses
func (h *handlers) PostExpense(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	username := getUsername(r)

	var req PostExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type != constant.TypeExpense && req.Type != constant.TypeIncome {
		jsonError(w, "type must be 'expense' or 'income'", http.StatusBadRequest)
		return
	}
	if req.Amount < constant.MinAmount {
		jsonError(w, fmt.Sprintf("amount must be >= %.2f", constant.MinAmount), http.StatusBadRequest)
		return
	}
	if req.LedgerID == 0 {
		jsonError(w, "ledger_id is required", http.StatusBadRequest)
		return
	}
	if len(req.Description) > constant.MaxDescriptionLength {
		jsonError(w, "description too long", http.StatusBadRequest)
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		jsonError(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	exp, err := h.expenseService.CreateExpense(userID, username, req.LedgerID, req.Type, req.Amount, req.Currency, req.CategoryCode, req.Description, date)
	if err != nil {
		logger.Error("PostExpense user=%d: %v", userID, err)
		jsonError(w, "failed to save expense", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, PostExpenseResponse{
		ID:           exp.ID,
		AmountInBase: exp.AmountInBase,
		BaseCurrency: h.rateService.BaseCurrency(),
	})

	if h.notifier != nil {
		icon := "💸"
		if req.Type == constant.TypeIncome {
			icon = "💰"
		}
		desc := ""
		if req.Description != "" {
			desc = " — " + req.Description
		}
		msg := fmt.Sprintf("%s Saved: %.2f %s (%.2f %s)%s", icon, req.Amount, req.Currency, exp.AmountInBase, h.rateService.BaseCurrency(), desc)
		go h.notifier.NotifyUser(userID, msg)
	}
}

// GET /api/expenses/latest
func (h *handlers) GetLatestExpense(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	exp, err := h.expenseService.GetMostRecentByUser(userID)
	if err != nil {
		jsonError(w, "no recent record found", http.StatusNotFound)
		return
	}

	ledgers, err := h.ledgerService.GetActiveByUserID(userID)
	if err != nil {
		logger.Error("GetLatestExpense ledgers user=%d: %v", userID, err)
	}
	ledgerName := ""
	for _, l := range ledgers {
		if l.ID == exp.LedgerID {
			ledgerName = l.Emoji + " " + l.Name
			break
		}
	}

	jsonOK(w, ExpenseResponse{
		ID:           exp.ID,
		LedgerID:     exp.LedgerID,
		LedgerName:   ledgerName,
		Type:         exp.Type,
		CategoryCode: exp.Category,
		Currency:     exp.Currency,
		Amount:       exp.Amount,
		AmountInBase: exp.AmountInBase,
		BaseCurrency: h.rateService.BaseCurrency(),
		Date:         exp.ExpenseDate.Format("2006-01-02"),
		Description:  exp.Description,
	})
}

// DELETE /api/expenses/{id}
func (h *handlers) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	idStr := r.PathValue("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid expense id", http.StatusBadRequest)
		return
	}
	if err := h.expenseService.DeleteExpense(uint(id), userID); err != nil {
		logger.Error("DeleteExpense user=%d id=%d: %v", userID, id, err)
		jsonError(w, "failed to delete expense", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	if h.notifier != nil {
		go h.notifier.NotifyUser(userID, "↩️ Record deleted.")
	}
}

// GET /api/stats?ledger_id=&start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	q := r.URL.Query()

	start, end, err := parseDateRange(q.Get("start"), q.Get("end"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var ledgerID uint
	if raw := q.Get("ledger_id"); raw != "" && raw != "0" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			jsonError(w, "invalid ledger_id", http.StatusBadRequest)
			return
		}
		ledgerID = uint(v)
	}

	expenses, err := h.expenseService.GetExpensesByLedgerAndDateRange(userID, ledgerID, start, end)
	if err != nil {
		logger.Error("GetStats user=%d: %v", userID, err)
		jsonError(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	ledgers, err := h.ledgerService.GetActiveByUserID(userID)
	if err != nil {
		logger.Error("GetStats ledgers user=%d: %v", userID, err)
	}
	ledgerName := make(map[uint]string, len(ledgers))
	for _, l := range ledgers {
		ledgerName[l.ID] = l.Emoji + " " + l.Name
	}

	jsonOK(w, buildStats(expenses, start, end, h.rateService.BaseCurrency(), ledgerName))
}

// GET /api/export?ledger_id=&start=YYYY-MM-DD&end=YYYY-MM-DD
func (h *handlers) GetExport(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	q := r.URL.Query()

	start, end, err := parseDateRange(q.Get("start"), q.Get("end"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	var ledgerID uint
	if raw := q.Get("ledger_id"); raw != "" && raw != "0" {
		v, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			jsonError(w, "invalid ledger_id", http.StatusBadRequest)
			return
		}
		ledgerID = uint(v)
	}

	expenses, err := h.expenseService.GetExpensesByLedgerAndDateRange(userID, ledgerID, start, end)
	if err != nil {
		logger.Error("GetExport user=%d: %v", userID, err)
		jsonError(w, "failed to get records", http.StatusInternalServerError)
		return
	}

	ledgers, err := h.ledgerService.GetActiveByUserID(userID)
	if err != nil {
		logger.Error("GetExport ledgers user=%d: %v", userID, err)
	}
	ledgerName := make(map[uint]string, len(ledgers))
	for _, l := range ledgers {
		ledgerName[l.ID] = l.Emoji + " " + l.Name
	}

	buf, err := buildExcel(expenses, ledgerName, h.rateService.BaseCurrency())
	if err != nil {
		logger.Error("GetExport build excel user=%d: %v", userID, err)
		jsonError(w, "failed to generate file", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("expenses_%s_to_%s.xlsx", start.Format("2006-01-02"), end.Format("2006-01-02"))
	jsonOK(w, map[string]string{"status": "ok"})

	if h.notifier != nil {
		caption := fmt.Sprintf("📊 Export: %s to %s (%d records)", start.Format("2006-01-02"), end.Format("2006-01-02"), len(expenses))
		go h.notifier.SendFileToUser(userID, filename, buf, caption)
	}
}

// GET /api/expenses/years
func (h *handlers) GetExpenseYears(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	years, err := h.expenseService.GetYears(userID)
	if err != nil {
		logger.Error("GetExpenseYears user=%d: %v", userID, err)
		jsonError(w, "failed to get years", http.StatusInternalServerError)
		return
	}
	if len(years) == 0 {
		jsonError(w, "No records yet. Start by adding your first record with /add.", http.StatusNotFound)
		return
	}
	jsonOK(w, years)
}

// GET /api/expenses/months?year=2024
func (h *handlers) GetExpenseMonths(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		jsonError(w, "invalid year", http.StatusBadRequest)
		return
	}
	months, err := h.expenseService.GetMonths(userID, year)
	if err != nil {
		logger.Error("GetExpenseMonths user=%d: %v", userID, err)
		jsonError(w, "failed to get months", http.StatusInternalServerError)
		return
	}
	jsonOK(w, months)
}

// GET /api/expenses?year=2024&month=1
func (h *handlers) GetExpenses(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	q := r.URL.Query()
	year, err := strconv.Atoi(q.Get("year"))
	if err != nil {
		jsonError(w, "invalid year", http.StatusBadRequest)
		return
	}
	month, err := strconv.Atoi(q.Get("month"))
	if err != nil {
		jsonError(w, "invalid month", http.StatusBadRequest)
		return
	}
	expenses, err := h.expenseService.GetByYearMonth(userID, year, month)
	if err != nil {
		logger.Error("GetExpenses user=%d: %v", userID, err)
		jsonError(w, "failed to get expenses", http.StatusInternalServerError)
		return
	}

	ledgers, err := h.ledgerService.GetActiveByUserID(userID)
	if err != nil {
		logger.Error("GetExpenses ledgers user=%d: %v", userID, err)
	}
	ledgerName := make(map[uint]string, len(ledgers))
	for _, l := range ledgers {
		ledgerName[l.ID] = l.Emoji + " " + l.Name
	}

	resp := make([]ExpenseResponse, 0, len(expenses))
	for _, exp := range expenses {
		resp = append(resp, ExpenseResponse{
			ID:           exp.ID,
			LedgerID:     exp.LedgerID,
			LedgerName:   ledgerName[exp.LedgerID],
			Type:         exp.Type,
			CategoryCode: exp.Category,
			Currency:     exp.Currency,
			Amount:       exp.Amount,
			AmountInBase: exp.AmountInBase,
			BaseCurrency: h.rateService.BaseCurrency(),
			Date:         exp.ExpenseDate.Format("2006-01-02"),
			Description:  exp.Description,
		})
	}
	jsonOK(w, resp)
}

// PUT /api/expenses/{id}
func (h *handlers) PutExpense(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid expense id", http.StatusBadRequest)
		return
	}

	var req PostExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Amount < constant.MinAmount {
		jsonError(w, fmt.Sprintf("amount must be >= %.2f", constant.MinAmount), http.StatusBadRequest)
		return
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		jsonError(w, "date must be YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	exp, err := h.expenseService.UpdateExpense(userID, uint(id), req.LedgerID, req.Type, req.Currency, req.CategoryCode, req.Description, req.Amount, date)
	if err != nil {
		logger.Error("PutExpense user=%d id=%d: %v", userID, id, err)
		jsonError(w, "failed to update expense", http.StatusInternalServerError)
		return
	}
	jsonOK(w, PostExpenseResponse{
		ID:           exp.ID,
		AmountInBase: exp.AmountInBase,
		BaseCurrency: h.rateService.BaseCurrency(),
	})
}

// POST /api/recurring
func (h *handlers) PostRecurring(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	username := getUsername(r)

	var req PostRecurringRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Type != constant.TypeExpense && req.Type != constant.TypeIncome {
		jsonError(w, "type must be 'expense' or 'income'", http.StatusBadRequest)
		return
	}
	if req.Amount < constant.MinAmount {
		jsonError(w, fmt.Sprintf("amount must be >= %.2f", constant.MinAmount), http.StatusBadRequest)
		return
	}
	if req.LedgerID == 0 {
		jsonError(w, "ledger_id is required", http.StatusBadRequest)
		return
	}
	if req.Frequency != "weekly" && req.Frequency != "monthly" {
		jsonError(w, "frequency must be 'weekly' or 'monthly'", http.StatusBadRequest)
		return
	}
	if req.Days == "" {
		jsonError(w, "days is required", http.StatusBadRequest)
		return
	}
	initialDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		initialDate = time.Now()
	}

	rec := &model.RecurringExpense{
		UserID:      userID,
		Username:    username,
		LedgerID:    req.LedgerID,
		Type:        req.Type,
		Amount:      req.Amount,
		Currency:    req.Currency,
		Category:    req.CategoryCode,
		Description: req.Description,
		Frequency:   req.Frequency,
		Days:        req.Days,
	}

	if err := h.recurringService.Create(rec, initialDate); err != nil {
		logger.Error("PostRecurring user=%d: %v", userID, err)
		jsonError(w, "failed to save recurring expense", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{"id": rec.ID, "next_trigger_at": rec.NextTriggerAt.Format("2006-01-02")})

	if h.notifier != nil {
		icon := "💸"
		if req.Type == constant.TypeIncome {
			icon = "💰"
		}
		msg := fmt.Sprintf("%s Recurring set: %.2f %s every %s (days: %s)\nFirst record saved today. Next: %s",
			icon, req.Amount, req.Currency, req.Frequency, req.Days, rec.NextTriggerAt.Format("2006-01-02"))
		go h.notifier.NotifyUser(userID, msg)
	}
}

func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("start must be YYYY-MM-DD")
	}
	end, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("end must be YYYY-MM-DD")
	}
	end = end.Add(24*time.Hour - time.Second)
	return start, end, nil
}

func buildStats(expenses []*model.Expense, start, end time.Time, baseCurrency string, ledgerName map[uint]string) StatsResponse {
	totalIncome := 0.0
	totalExpense := 0.0
	categoryTotals := make(map[string]float64)
	ledgerTotals := make(map[uint]float64)

	for _, exp := range expenses {
		if exp.Type == constant.TypeIncome {
			totalIncome += exp.AmountInBase
		} else {
			totalExpense += exp.AmountInBase
			categoryTotals[exp.Category] += exp.AmountInBase
			ledgerTotals[exp.LedgerID] += exp.AmountInBase
		}
	}

	catItems := make([]StatsCategoryItem, 0, len(categoryTotals))
	for cat, amount := range categoryTotals {
		pct := 0.0
		if totalExpense > 0 {
			pct = amount / totalExpense * 100
		}
		catItems = append(catItems, StatsCategoryItem{Category: cat, Amount: amount, Percentage: pct})
	}

	ledgerItems := make([]StatsLedgerItem, 0, len(ledgerTotals))
	for id, amount := range ledgerTotals {
		pct := 0.0
		if totalExpense > 0 {
			pct = amount / totalExpense * 100
		}
		name := ledgerName[id]
		if name == "" {
			name = "Unknown"
		}
		ledgerItems = append(ledgerItems, StatsLedgerItem{LedgerName: name, Amount: amount, Percentage: pct})
	}

	return StatsResponse{
		Period:       start.Format("2006-01-02") + " to " + end.Format("2006-01-02"),
		BaseCurrency: baseCurrency,
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		Net:          totalIncome - totalExpense,
		ByCategory:   catItems,
		ByLedger:     ledgerItems,
	}
}

func buildExcel(expenses []*model.Expense, ledgerName map[uint]string, baseCurrency string) ([]byte, error) {
	xlsx := excelize.NewFile()
	sheet := "Records"
	xlsx.SetSheetName("Sheet1", sheet)

	headers := []string{"Date", "Username", "Ledger", "Type", "Category", "Amount", "Currency", "Amount(" + baseCurrency + ")", "Description"}
	for i, h := range headers {
		xlsx.SetCellValue(sheet, fmt.Sprintf("%c1", 'A'+i), h)
	}

	for i, exp := range expenses {
		row := i + 2
		xlsx.SetCellValue(sheet, fmt.Sprintf("A%d", row), exp.ExpenseDate.Format("2006-01-02"))
		xlsx.SetCellValue(sheet, fmt.Sprintf("B%d", row), exp.Username)
		xlsx.SetCellValue(sheet, fmt.Sprintf("C%d", row), ledgerName[exp.LedgerID])
		xlsx.SetCellValue(sheet, fmt.Sprintf("D%d", row), exp.Type)
		xlsx.SetCellValue(sheet, fmt.Sprintf("E%d", row), exp.Category)
		xlsx.SetCellValue(sheet, fmt.Sprintf("F%d", row), exp.Amount)
		xlsx.SetCellValue(sheet, fmt.Sprintf("G%d", row), exp.Currency)
		xlsx.SetCellValue(sheet, fmt.Sprintf("H%d", row), exp.AmountInBase)
		xlsx.SetCellValue(sheet, fmt.Sprintf("I%d", row), exp.Description)
	}

	var buf bytes.Buffer
	if err := xlsx.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
