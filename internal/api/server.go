package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"

	"fin-bot-miniapp/internal/config"
	"fin-bot-miniapp/internal/service"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(
	expenseService *service.ExpenseService,
	categoryService *service.CategoryService,
	ledgerService *service.LedgerService,
	rateService *service.RateService,
	recurringService *service.RecurringService,
	botToken string,
	cfg config.ServerConfig,
	webFS fs.FS,
	notifier Notifier,
) *Server {
	h := &handlers{
		expenseService:   expenseService,
		categoryService:  categoryService,
		ledgerService:    ledgerService,
		rateService:      rateService,
		recurringService: recurringService,
		notifier:         notifier,
	}

	mux := http.NewServeMux()

	// Static frontend
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	// API routes — all protected by initData middleware
	auth := InitDataMiddleware(botToken, cfg.InitDataTTL)

	mux.Handle("GET /api/currencies", auth(http.HandlerFunc(h.GetCurrencies)))
	mux.Handle("GET /api/expenses/years", auth(http.HandlerFunc(h.GetExpenseYears)))
	mux.Handle("GET /api/expenses/months", auth(http.HandlerFunc(h.GetExpenseMonths)))
	mux.Handle("GET /api/expenses", auth(http.HandlerFunc(h.GetExpenses)))
	mux.Handle("PUT /api/expenses/{id}", auth(http.HandlerFunc(h.PutExpense)))
	mux.Handle("GET /api/categories", auth(http.HandlerFunc(h.GetCategories)))
	mux.Handle("GET /api/ledgers", auth(http.HandlerFunc(h.GetLedgers)))
	mux.Handle("POST /api/expenses", auth(http.HandlerFunc(h.PostExpense)))
	mux.Handle("GET /api/expenses/latest", auth(http.HandlerFunc(h.GetLatestExpense)))
	mux.Handle("DELETE /api/expenses/{id}", auth(http.HandlerFunc(h.DeleteExpense)))
	mux.Handle("GET /api/stats", auth(http.HandlerFunc(h.GetStats)))
	mux.Handle("GET /api/export", auth(http.HandlerFunc(h.GetExport)))
	mux.Handle("POST /api/recurring", auth(http.HandlerFunc(h.PostRecurring)))

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Addr,
			Handler: CORSMiddleware(mux),
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}
