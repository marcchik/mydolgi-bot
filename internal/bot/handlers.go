package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/yourname/dolgo-bot/internal/config"
	"github.com/yourname/dolgo-bot/internal/repo"
)

type Handler struct {
	api *tgbotapi.BotAPI
	cfg config.Config

	users    *repo.Users
	contacts *repo.Contacts
	debts    *repo.Debts

	reminderTick time.Time
}

func NewHandler(api *tgbotapi.BotAPI, cfg config.Config, u *repo.Users, c *repo.Contacts, d *repo.Debts) *Handler {
	return &Handler{api: api, cfg: cfg, users: u, contacts: c, debts: d}
}

func (h *Handler) HandleUpdate(ctx context.Context, upd tgbotapi.Update) {
	if upd.CallbackQuery != nil {
		h.HandleCallback(ctx, upd.CallbackQuery)
		return
	}

	if upd.Message == nil {
		return
	}

	msg := upd.Message
	// работаем только в личке
	if !msg.Chat.IsPrivate() {
		// Можно отвечать подсказкой, но лучше молчать/минимум.
		return
	}

	// Ensure registration (upsert)
	var uname *string
	if msg.From.UserName != "" {
		u := msg.From.UserName
		uname = &u
	}
	var fn *string
	if msg.From.FirstName != "" {
		s := msg.From.FirstName
		fn = &s
	}
	var ln *string
	if msg.From.LastName != "" {
		s := msg.From.LastName
		ln = &s
	}

	ownerID, err := h.users.UpsertTelegramUser(ctx, msg.From.ID, uname, fn, ln)
	if err != nil {
		log.Printf("upsert user: %v", err)
		return
	}

	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/start") {
		h.reply(msg.Chat.ID, "Привет! Я DolgoBot.\n\nКоманды:\n/add @username — добавить контакт\n/alias @username Имя Фамилия — алиас\n\nЧтобы записать долг просто напиши:\n`300$ Антон 12.12.2025`\nили\n`300$ Антон Потупчик 12 декабря 2025`", true)
		return
	}

	if strings.HasPrefix(text, "/add") {
		h.handleAdd(ctx, msg.Chat.ID, ownerID, text)
		return
	}

	if strings.HasPrefix(text, "/alias") {
		h.handleAlias(ctx, msg.Chat.ID, ownerID, text)
		return
	}

	if strings.HasPrefix(text, "/debtors") {
		h.handleDebtors(ctx, msg.Chat.ID, ownerID)
		return
	}

	if strings.HasPrefix(text, "/mydebts") {
		h.handleMyDebts(ctx, msg.Chat.ID, ownerID)
		return
	}

	if strings.HasPrefix(text, "/debts") {
		h.handleSummary(ctx, msg.Chat.ID, ownerID)
		return
	}

	if strings.HasPrefix(text, "/contacts") {
		h.handleContactsInline(ctx, msg.Chat.ID, ownerID)
		return
	}

	if strings.HasPrefix(text, "/paid") || strings.HasPrefix(text, "/close") {
		h.handlePaid(ctx, msg.Chat.ID, ownerID, text)
		return
	}

	// Default: try parse as debt record
	parsed, err := ParseDebtText(text)
	if err != nil {
		h.reply(msg.Chat.ID, "❌ "+err.Error(), false)
		return
	}
	if parsed.RawName == "" {
		h.reply(msg.Chat.ID, "❌ Не понял, кому записать. Сначала добавь контакт: /add @username", false)
		return
	}

	debtorID, candidates, err := h.contacts.FindContactByConfirmingName(ctx, ownerID, parsed.RawName)
	if err != nil {
		h.reply(msg.Chat.ID, "❌ Ошибка поиска контакта", false)
		return
	}

	if debtorID == 0 {
		if len(candidates) == 0 {
			h.reply(msg.Chat.ID, "❌ Не нашёл такого контакта в твоём списке.\nДобавь: /add @username\nПотом задай алиас: /alias @username Антон Потупчик", false)
			return
		}
		// ambiguous: list candidates
		var b strings.Builder
		b.WriteString("Я нашёл несколько вариантов, уточни алиасом:\n")
		for i, c := range candidates {
			display := strings.TrimSpace(strings.Join([]string{c.FirstName, c.LastName}, " "))
			if display == "" && c.Username != "" {
				display = "@" + c.Username
			}
			if display == "" {
				display = fmt.Sprintf("user_id=%d", c.UserID)
			}
			b.WriteString(fmt.Sprintf("%d) %s\n", i+1, display))
		}
		b.WriteString("\nСделай более точный алиас через /alias")
		h.reply(msg.Chat.ID, b.String(), false)
		return
	}

	debtID, err := h.debts.CreateDebt(ctx, ownerID, debtorID, parsed.AmountCents, parsed.Currency, parsed.DueDate)
	if err != nil {
		h.reply(msg.Chat.ID, "❌ Не удалось записать долг (БД)", false)
		return
	}

	// notify both
	amount := formatMoney(parsed.AmountCents, parsed.Currency)
	due := parsed.DueDate.Format("02.01.2006")

	h.reply(msg.Chat.ID, fmt.Sprintf("✅ Записал долг #%d\nТы одолжил: %s\nКому: %s\nСрок: %s", debtID, amount, parsed.RawName, due), false)

	// notify debtor
	debtorTg, err := h.users.GetTelegramIDByUserID(ctx, debtorID)
	if err == nil {
		h.sendDM(debtorTg, fmt.Sprintf("📌 Тебе записали долг: %s\nСрок: %s\n(кредитор: @%s)", amount, due, safeUsername(msg.From.UserName)))
	}
}

func (h *Handler) handleAdd(ctx context.Context, chatID int64, ownerID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		h.reply(chatID, "Используй: /add @username", false)
		return
	}
	u := strings.TrimSpace(parts[1])
	u = strings.TrimPrefix(u, "@")
	if u == "" {
		h.reply(chatID, "Используй: /add @username", false)
		return
	}

	// В Telegram Bot API нельзя по username получить telegram_id напрямую.
	// Поэтому "add" делаем через “взаимную регистрацию”: друг должен написать /start боту.
	// Затем ты добавляешь его по @username, а мы ищем user_id в нашей БД (users.username).
	var contactID int64
	contactID, err := h.users.FindByUsername(ctx, u)

	if err != nil {
		h.reply(chatID, "❌ Я не знаю этого пользователя. Попроси его написать мне /start, а потом повтори /add @username", false)
		return
	}

	if err := h.contacts.AddContact(ctx, ownerID, contactID); err != nil {
		h.reply(chatID, "❌ Не удалось добавить контакт", false)
		return
	}
	// default aliases: username
	_ = h.contacts.AddAlias(ctx, ownerID, contactID, u)

	h.reply(chatID, fmt.Sprintf("✅ Контакт добавлен: @%s\nТеперь можешь задать алиас:\n/alias @%s Антон Потупчик", u, u), false)
}

func (h *Handler) handleAlias(ctx context.Context, chatID int64, ownerID int64, text string) {
	// /alias @user Имя Фамилия
	rest := strings.TrimSpace(strings.TrimPrefix(text, "/alias"))
	if rest == "" {
		h.reply(chatID, "Используй: /alias @username Антон Потупчик", false)
		return
	}

	parts := strings.Fields(rest)
	if len(parts) < 2 {
		h.reply(chatID, "Используй: /alias @username Антон Потупчик", false)
		return
	}

	u := strings.TrimPrefix(parts[0], "@")
	alias := strings.TrimSpace(strings.Join(parts[1:], " "))
	if u == "" || alias == "" {
		h.reply(chatID, "Используй: /alias @username Антон Потупчик", false)
		return
	}

	contactID, err := h.users.FindByUsername(ctx, u)
	if err != nil {
		h.reply(chatID, "❌ Я не знаю этого пользователя. Пусть он напишет /start боту.", false)
		return
	}

	// Ensure contact exists
	_ = h.contacts.AddContact(ctx, ownerID, contactID)

	if err := h.contacts.AddAlias(ctx, ownerID, contactID, alias); err != nil {
		h.reply(chatID, "❌ Не удалось сохранить алиас", false)
		return
	}

	h.reply(chatID, fmt.Sprintf("✅ Алиас сохранён: %q → @%s", alias, u), false)
}

func (h *Handler) sendDM(telegramID int64, text string) {
	msg := tgbotapi.NewMessage(telegramID, text)
	_, _ = h.api.Send(msg)
}

func formatMoney(cents int64, cur string) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := cents / 100
	fr := cents % 100
	return fmt.Sprintf("%s%d.%02d %s", sign, whole, fr, cur)
}

func safeUsername(u string) string {
	if u == "" {
		return "unknown"
	}
	return u
}

func (h *Handler) RunReminderWorker(ctx context.Context, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1) обновляем просрочку
			_ = h.debts.MarkOverdue(ctx)

			// 2) шлём напоминания на due_date-offset
			for _, offset := range h.cfg.RemindDaysBefore {
				debts, err := h.debts.GetDebtsDueOnOffset(ctx, offset)
				if err != nil {
					continue
				}
				for _, d := range debts {
					amount := formatMoney(d.AmountCents, d.Currency)
					when := d.DueDate.Format("02.01.2006")
					msg := ""
					if offset > 0 {
						msg = fmt.Sprintf("⏰ Напоминание: через %d дн. срок долга #%d\n%s до %s", offset, d.ID, amount, when)
					} else {
						msg = fmt.Sprintf("⏰ Сегодня срок долга #%d\n%s до %s", d.ID, amount, when)
					}

					// кредитору
					if tg, e := h.users.GetTelegramIDByUserID(ctx, d.CreditorID); e == nil {
						h.sendDM(tg, "Кредитору:\n"+msg)
					}
					// должнику
					if tg, e := h.users.GetTelegramIDByUserID(ctx, d.DebtorID); e == nil {
						h.sendDM(tg, "Должнику:\n"+msg)
					}
				}
			}
		}
	}
}

func (h *Handler) reply(chatID int64, text string, markdown bool) {
	msg := tgbotapi.NewMessage(chatID, text)
	if markdown {
		msg.ParseMode = "Markdown"
	}
	_, _ = h.api.Send(msg)
}

func (h *Handler) handleContactsInline(ctx context.Context, chatID int64, ownerID int64) {
	contacts, err := h.contacts.ListContactsWithAliases(ctx, ownerID, 100)
	if err != nil {
		h.reply(chatID, "❌ Не удалось получить контакты (БД)", false)
		return
	}

	if len(contacts) == 0 {
		h.reply(chatID, "👥 Контактов пока нет.\nДобавь: /add @username", false)
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton

	for _, c := range contacts {
		title := c.Username
		if title == "" {
			title = strings.TrimSpace(c.FirstName + " " + c.LastName)
		}
		if title == "" {
			title = fmt.Sprintf("user_id=%d", c.UserID)
		}

		btn := tgbotapi.NewInlineKeyboardButtonData(
			"⚙️ "+title,
			fmt.Sprintf("contact:%d", c.UserID),
		)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{btn})
	}

	msg := tgbotapi.NewMessage(chatID, "👥 *Твои контакты:*")
	msg.ParseMode = "Markdown"
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)

	h.api.Send(msg)
}
func (h *Handler) HandleCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	data := q.Data

	// обязательно отвечаем Telegram
	defer h.api.Request(tgbotapi.NewCallback(q.ID, ""))

	// 🔹 КНОПКИ БЕЗ :
	if data == "back_contacts" {
		h.editContactsMenu(ctx, q)
		return
	}

	parts := strings.Split(data, ":")
	if len(parts) < 2 {
		return
	}

	switch parts[0] {

	case "contact":
		contactID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.showContactMenu(ctx, q, contactID)

	case "contact_delete":
		contactID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.deleteContact(ctx, q, contactID)

	case "contact_aliases":
		contactID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.showContactAliases(ctx, q, contactID)

	case "alias_delete":
		aliasID, _ := strconv.ParseInt(parts[1], 10, 64)
		h.deleteAlias(ctx, q, aliasID)
	}
}

func (h *Handler) showContactMenu(ctx context.Context, q *tgbotapi.CallbackQuery, contactID int64) {
	text := "Что сделать с контактом?"

	kb := tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("📛 Алиасы", fmt.Sprintf("contact_aliases:%d", contactID)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🗑 Удалить", fmt.Sprintf("contact_delete:%d", contactID)),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "back_contacts"),
		},
	)

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		text,
	)
	edit.ReplyMarkup = &kb

	h.api.Send(edit)
}
func (h *Handler) deleteContact(ctx context.Context, q *tgbotapi.CallbackQuery, contactID int64) {
	// узнаём owner
	ownerID, err := h.users.GetUserIDByTelegramID(ctx, q.From.ID)
	if err != nil {
		return
	}

	err = h.contacts.DeleteContact(ctx, ownerID, contactID)
	if err != nil {
		h.api.Send(tgbotapi.NewEditMessageText(
			q.Message.Chat.ID,
			q.Message.MessageID,
			"❌ Не удалось удалить контакт",
		))
		return
	}

	h.api.Send(tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		"✅ Контакт удалён",
	))
}
