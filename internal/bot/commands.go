package bot

import (
	"fmt"
	"strings"

	"fin-bot-miniapp/internal/logger"

	tele "gopkg.in/telebot.v3"
)

func (h *Handler) handleStart(c tele.Context) error {
	return c.Send("👋 Welcome to Finance Bot!\n\nUse /add to record an expense or income.\nType /help for all commands.")
}

func (h *Handler) handleAdd(c tele.Context) error {
	if h.isGroup(c) {
		return c.Send("Please message me privately to add records: @" + h.bot.Me.Username)
	}
	markup := &tele.ReplyMarkup{}
	btn := markup.WebApp("📝 Add Record", &tele.WebApp{URL: h.miniAppURL + "/app/add.html"})
	markup.Inline(markup.Row(btn))
	return c.Send("Tap the button to open the add form:", markup)
}

func (h *Handler) handleStats(c tele.Context) error {
	if h.isGroup(c) {
		return c.Send("Please message me privately to view statistics: @" + h.bot.Me.Username)
	}
	markup := &tele.ReplyMarkup{}
	btn := markup.WebApp("📊 View Statistics", &tele.WebApp{URL: h.miniAppURL + "/app/stats.html"})
	markup.Inline(markup.Row(btn))
	return c.Send("Tap the button to view statistics:", markup)
}

func (h *Handler) handleExport(c tele.Context) error {
	if h.isGroup(c) {
		return c.Send("Please message me privately to export records: @" + h.bot.Me.Username)
	}
	markup := &tele.ReplyMarkup{}
	btn := markup.WebApp("📋 Export to Excel", &tele.WebApp{URL: h.miniAppURL + "/app/export.html"})
	markup.Inline(markup.Row(btn))
	return c.Send("Tap the button to export records:", markup)
}

func (h *Handler) handleHistory(c tele.Context) error {
	if h.isGroup(c) {
		return c.Send("Please message me privately to view history: @" + h.bot.Me.Username)
	}
	markup := &tele.ReplyMarkup{}
	btn := markup.WebApp("📜 View History", &tele.WebApp{URL: h.miniAppURL + "/app/history.html"})
	markup.Inline(markup.Row(btn))
	return c.Send("Tap the button to browse your records:", markup)
}

func (h *Handler) handleUndo(c tele.Context) error {
	if h.isGroup(c) {
		return c.Send("Please message me privately to undo records: @" + h.bot.Me.Username)
	}
	markup := &tele.ReplyMarkup{}
	btn := markup.WebApp("↩️ Undo Last Record", &tele.WebApp{URL: h.miniAppURL + "/app/undo.html"})
	markup.Inline(markup.Row(btn))
	return c.Send("Tap the button to review and undo:", markup)
}

func (h *Handler) handleAddCat(c tele.Context) error {
	args := c.Args()
	if len(args) < 3 {
		return c.Send("Usage: /addcat <code> <emoji> <name>\nExample: /addcat gym 🏋️ Gym")
	}
	code := args[0]
	emoji := args[1]
	name := strings.Join(args[2:], " ")

	userID := c.Sender().ID
	if err := h.categoryService.AddCategory(userID, code, name, emoji, 99); err != nil {
		logger.Error("AddCategory user=%d: %v", userID, err)
		return c.Send(fmt.Sprintf("❌ Failed to add category: %v", err))
	}
	return c.Send(fmt.Sprintf("✅ Category added: %s %s", emoji, name))
}

func (h *Handler) handleAddLedger(c tele.Context) error {
	args := c.Args()
	if len(args) < 3 {
		return c.Send("Usage: /addledger <code> <emoji> <name>\nExample: /addledger savings 💰 Savings")
	}
	code := args[0]
	emoji := args[1]
	name := strings.Join(args[2:], " ")

	userID := c.Sender().ID
	if err := h.ledgerService.AddLedger(userID, code, name, emoji, 99); err != nil {
		logger.Error("AddLedger user=%d: %v", userID, err)
		return c.Send(fmt.Sprintf("❌ Failed to add ledger: %v", err))
	}
	return c.Send(fmt.Sprintf("✅ Ledger added: %s %s", emoji, name))
}

func (h *Handler) handleHelp(c tele.Context) error {
	return c.Send(`Finance Bot — Commands

/add        Record an expense or income
/stats      View statistics
/export     Export records to Excel
/undo       Undo the last record
/history    Browse and edit past records
/addcat     Add a custom category
/addledger  Add a custom ledger
/help       Show this message`)
}
