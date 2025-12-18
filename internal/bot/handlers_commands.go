package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

func (h *Handler) handleDebtors(ctx context.Context, chatID int64, ownerID int64) {
	rows, err := h.debts.ListDebtors(ctx, ownerID, 50)
	if err != nil {
		log.Printf("ListDebtors error: %v", err)
		h.reply(chatID, "❌ Не удалось получить список должников (БД)", false)
		return
	}
	if len(rows) == 0 {
		h.reply(chatID, "📥 Тебе сейчас никто не должен 👍", false)
		return
	}

	var b strings.Builder
	b.WriteString("📥 *Тебе должны:*\n\n")

	for _, d := range rows {
		b.WriteString(fmt.Sprintf(
			"#%d %s — %s (до %s)\n",
			d.ID,
			displayName(d.Name),
			formatMoney(d.AmountCents, d.Currency),
			d.DueDate.Format("02.01.2006"),
		))
	}

	b.WriteString("\nЗакрыть долг: `/paid <id>`")
	h.reply(chatID, b.String(), true)
}

func (h *Handler) handleMyDebts(ctx context.Context, chatID int64, ownerID int64) {
	rows, err := h.debts.ListMyDebts(ctx, ownerID, 50)
	if err != nil {
		h.reply(chatID, "❌ Не удалось получить список твоих долгов (БД)", false)
		return
	}
	if len(rows) == 0 {
		h.reply(chatID, "📤 Ты сейчас никому не должен 👍", false)
		return
	}

	var b strings.Builder
	b.WriteString("📤 *Ты должен:*\n\n")

	for _, d := range rows {
		b.WriteString(fmt.Sprintf(
			"#%d %s — %s (до %s)\n",
			d.ID,
			displayName(d.Name),
			formatMoney(d.AmountCents, d.Currency),
			d.DueDate.Format("02.01.2006"),
		))
	}

	b.WriteString("\nЗакрыть долг: `/paid <id>`")
	h.reply(chatID, b.String(), true)
}

func (h *Handler) handleSummary(ctx context.Context, chatID int64, ownerID int64) {
	rows, err := h.debts.SummaryByCurrency(ctx, ownerID)
	if err != nil {
		h.reply(chatID, "❌ Не удалось получить сводку (БД)", false)
		return
	}
	if len(rows) == 0 {
		h.reply(chatID, "📊 Пока нет активных долгов.", false)
		return
	}

	var b strings.Builder
	b.WriteString("📊 *Сводка по валютам (активные долги):*\n\n")
	for _, s := range rows {
		b.WriteString(fmt.Sprintf("*%s*\n", s.Currency))
		b.WriteString(fmt.Sprintf("  Ты одолжил: %s\n", formatMoney(s.YouLentCents, s.Currency)))
		b.WriteString(fmt.Sprintf("  Ты должен:  %s\n", formatMoney(s.YouOweCents, s.Currency)))
		net := s.NetCents
		sign := "+"
		if net < 0 {
			sign = "-"
			net = -net
		}
		b.WriteString(fmt.Sprintf("  Баланс:     %s%s\n\n", sign, formatMoney(net, s.Currency)))
	}
	h.reply(chatID, b.String(), true)
}

func (h *Handler) handleContacts(ctx context.Context, chatID int64, ownerID int64) {
	contacts, err := h.contacts.ListContactsWithAliases(ctx, ownerID, 200)
	if err != nil {
		h.reply(chatID, "❌ Не удалось получить контакты (БД)", false)
		return
	}
	if len(contacts) == 0 {
		h.reply(chatID, "👥 Контактов пока нет.\nДобавь: /add @username", false)
		return
	}

	var b strings.Builder
	b.WriteString("👥 *Твои контакты:*\n\n")
	for _, c := range contacts {
		title := ""
		if c.Username != "" {
			title = "@" + c.Username
		} else {
			title = strings.TrimSpace(strings.Join([]string{c.FirstName, c.LastName}, " "))
			if title == "" {
				title = fmt.Sprintf("user_id=%d", c.UserID)
			}
		}
		b.WriteString(fmt.Sprintf("*%s*\n", escapeMD(title)))
		if len(c.Aliases) > 0 {
			for _, a := range uniqueStrings(c.Aliases) {
				if strings.TrimSpace(a) == "" {
					continue
				}
				b.WriteString(fmt.Sprintf("  └ %s\n", escapeMD(a)))
			}
		} else {
			b.WriteString("  └ (нет алиасов)\n")
		}
		b.WriteString("\n")
	}
	h.reply(chatID, b.String(), true)
}

func (h *Handler) handlePaid(ctx context.Context, chatID int64, ownerID int64, text string) {
	parts := strings.Fields(text)
	if len(parts) < 2 {
		h.reply(chatID, "Используй: /paid <id>\nПример: /paid 12", false)
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		h.reply(chatID, "❌ Неверный id долга. Пример: /paid 12", false)
		return
	}

	ok, err := h.debts.CloseDebt(ctx, ownerID, id)
	if err != nil {
		h.reply(chatID, "❌ Не удалось закрыть долг (БД)", false)
		return
	}
	if !ok {
		h.reply(chatID, "❌ Долг не найден или уже закрыт (или не твой).", false)
		return
	}

	h.reply(chatID, fmt.Sprintf("✅ Долг #%d закрыт", id), false)
}

func displayName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	return s
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		low := strings.ToLower(v)
		if _, ok := seen[low]; ok {
			continue
		}
		seen[low] = struct{}{}
		out = append(out, v)
	}
	return out
}

// очень простой escape для Markdown (минимально нужное)
func escapeMD(s string) string {
	repl := []struct{ a, b string }{
		{"_", "\\_"},
		{"*", "\\*"},
		{"[", "\\["},
		{"]", "\\]"},
		{"(", "\\("},
		{")", "\\)"},
		{"`", "\\`"},
	}
	for _, r := range repl {
		s = strings.ReplaceAll(s, r.a, r.b)
	}
	return s
}

var _ = time.Second // чтобы не ругался импорт time, если у тебя уже есть где-то
