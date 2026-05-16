package api

type PostExpenseRequest struct {
	LedgerID     uint    `json:"ledger_id"`
	Type         string  `json:"type"`
	CategoryCode string  `json:"category_code"`
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	Date         string  `json:"date"` // YYYY-MM-DD
	Description  string  `json:"description"`
}

type PostExpenseResponse struct {
	ID           uint    `json:"id"`
	AmountInBase float64 `json:"amount_in_base"`
	BaseCurrency string  `json:"base_currency"`
}

type CategoryResponse struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

type LedgerResponse struct {
	ID    uint   `json:"id"`
	Code  string `json:"code"`
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
}

type ExpenseResponse struct {
	ID           uint    `json:"id"`
	LedgerID     uint    `json:"ledger_id"`
	LedgerName   string  `json:"ledger_name"`
	Type         string  `json:"type"`
	CategoryCode string  `json:"category_code"`
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	AmountInBase float64 `json:"amount_in_base"`
	BaseCurrency string  `json:"base_currency"`
	Date         string  `json:"date"`
	Description  string  `json:"description"`
}

type StatsCategoryItem struct {
	Category   string  `json:"category"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type StatsLedgerItem struct {
	LedgerName string  `json:"ledger_name"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type StatsResponse struct {
	Period       string            `json:"period"`
	BaseCurrency string            `json:"base_currency"`
	TotalIncome  float64           `json:"total_income"`
	TotalExpense float64           `json:"total_expense"`
	Net          float64           `json:"net"`
	ByCategory   []StatsCategoryItem `json:"by_category"`
	ByLedger     []StatsLedgerItem   `json:"by_ledger"`
}

type PostRecurringRequest struct {
	LedgerID     uint    `json:"ledger_id"`
	Type         string  `json:"type"`
	CategoryCode string  `json:"category_code"`
	Currency     string  `json:"currency"`
	Amount       float64 `json:"amount"`
	Date         string  `json:"date"` // YYYY-MM-DD, date of the first record
	Description  string  `json:"description"`
	Frequency    string  `json:"frequency"` // "weekly" or "monthly"
	Days         string  `json:"days"`      // e.g. "1,3,5" or "1,15"
}

type ErrorResponse struct {
	Error string `json:"error"`
}
