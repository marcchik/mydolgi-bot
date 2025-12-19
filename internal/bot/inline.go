package bot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) HandleInlineQuery(ctx context.Context, q *tgbotapi.InlineQuery) {
	if q.Query == "" {
		return
	}

	parsed, err := ParseDebtText(q.Query)
	if err != nil || parsed.RawName == "" {
		return
	}

	creditorID, err := h.users.GetUserIDByTelegramID(ctx, q.From.ID)
	if err != nil {
		return
	}

	debtorID, _, err := h.contacts.FindContactByConfirmingName(
		ctx,
		creditorID,
		parsed.RawName,
	)
	if err != nil || debtorID == 0 {
		return
	}

	debtID, err := h.debts.CreateDebt(
		ctx,
		creditorID,
		debtorID,
		parsed.AmountCents,
		parsed.Currency,
		parsed.DueDate,
	)
	if err != nil {
		return
	}

	amount := formatMoney(parsed.AmountCents, parsed.Currency)
	due := parsed.DueDate.Format("02.01.2006")

	article := tgbotapi.NewInlineQueryResultArticle(
		fmt.Sprintf("debt_%d", debtID),
		"📌 Зафиксировать долг",
		fmt.Sprintf(
			"📌 Долг зафиксирован\n\n%s → %s\nСрок: %s\nID: #%d",
			amount,
			parsed.RawName,
			due,
			debtID,
		),
	)

	article.Description = fmt.Sprintf(
		"%s → %s до %s",
		amount,
		parsed.RawName,
		due,
	)
	cfg := tgbotapi.InlineConfig{
		InlineQueryID: q.ID,
		Results:       []interface{}{article},
		IsPersonal:    true,
		CacheTime:     0,
	}

	_, _ = h.api.Request(cfg)

	h.notifyDebtCreated(ctx, creditorID, debtorID, amount, due)
}

func (h *Handler) notifyDebtCreated(
	ctx context.Context,
	creditorID, debtorID int64,
	amount, due string,
) {
	if tg, err := h.users.GetTelegramIDByUserID(ctx, creditorID); err == nil {
		h.sendDM(tg, fmt.Sprintf("✅ Ты зафиксировал долг\n%s до %s", amount, due))
	}
	if tg, err := h.users.GetTelegramIDByUserID(ctx, debtorID); err == nil {
		h.sendDM(tg, fmt.Sprintf("📌 Тебе записали долг\n%s до %s", amount, due))
	}
}
