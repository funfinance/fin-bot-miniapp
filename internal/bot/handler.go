package bot

import (
	"bytes"
	"strings"

	"fin-bot-miniapp/internal/logger"
	"fin-bot-miniapp/internal/service"

	tele "gopkg.in/telebot.v3"
)

type Handler struct {
	bot             *tele.Bot
	expenseService  *service.ExpenseService
	categoryService *service.CategoryService
	ledgerService   *service.LedgerService
	rateService     *service.RateService
	miniAppURL      string
}

func NewHandler(
	bot *tele.Bot,
	expenseService *service.ExpenseService,
	categoryService *service.CategoryService,
	ledgerService *service.LedgerService,
	rateService *service.RateService,
	miniAppURL string,
) *Handler {
	return &Handler{
		bot:             bot,
		expenseService:  expenseService,
		categoryService: categoryService,
		ledgerService:   ledgerService,
		rateService:     rateService,
		miniAppURL:      strings.TrimRight(miniAppURL, "/"),
	}
}

func (h *Handler) Register() {
	h.bot.Use(h.recoverMiddleware)
	h.bot.Use(h.errorMiddleware)

	h.bot.Handle("/start", h.handleStart)
	h.bot.Handle("/add", h.handleAdd)
	h.bot.Handle("/stats", h.handleStats)
	h.bot.Handle("/export", h.handleExport)
	h.bot.Handle("/undo", h.handleUndo)
	h.bot.Handle("/history", h.handleHistory)
	h.bot.Handle("/addcat", h.handleAddCat)
	h.bot.Handle("/addledger", h.handleAddLedger)
	h.bot.Handle("/help", h.handleHelp)
}

func (h *Handler) NotifyUser(userID int64, msg string) {
	if _, err := h.bot.Send(tele.ChatID(userID), msg); err != nil {
		logger.Error("NotifyUser user=%d: %v", userID, err)
	}
}

func (h *Handler) SendFileToUser(userID int64, filename string, data []byte, caption string) {
	doc := &tele.Document{
		File:     tele.FromReader(bytes.NewReader(data)),
		FileName: filename,
		Caption:  caption,
	}
	if _, err := h.bot.Send(tele.ChatID(userID), doc); err != nil {
		logger.Error("SendFileToUser user=%d: %v", userID, err)
	}
}

func (h *Handler) isGroup(c tele.Context) bool {
	t := c.Chat().Type
	return t == tele.ChatGroup || t == tele.ChatSuperGroup
}
