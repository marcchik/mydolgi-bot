package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) editContactsMenu(ctx context.Context, q *tgbotapi.CallbackQuery) {
	ownerID, err := h.users.GetUserIDByTelegramID(ctx, q.From.ID)
	if err != nil {
		return
	}

	contacts, err := h.contacts.ListContactsWithAliases(ctx, ownerID, 100)
	if err != nil {
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

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		"👥 *Твои контакты:*",
	)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: rows,
	}

	h.api.Send(edit)
}

func (h *Handler) showContactAliases(ctx context.Context, q *tgbotapi.CallbackQuery, contactID int64) {
	ownerID, err := h.users.GetUserIDByTelegramID(ctx, q.From.ID)
	if err != nil {
		return
	}

	aliases, err := h.contacts.ListAliases(ctx, ownerID, contactID)
	if err != nil {
		return
	}

	var text strings.Builder
	text.WriteString("📛 *Алиасы контакта:*\n\n")

	var rows [][]tgbotapi.InlineKeyboardButton

	if len(aliases) == 0 {
		text.WriteString("— алиасов нет —\n")
	} else {
		for _, a := range aliases {
			text.WriteString("• " + escapeMD(a.Value) + "\n")
			rows = append(rows, []tgbotapi.InlineKeyboardButton{
				tgbotapi.NewInlineKeyboardButtonData(
					"❌ "+a.Value,
					fmt.Sprintf("alias_delete:%d", a.ID),
				),
			})
		}
	}

	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("contact:%d", contactID)),
	})

	edit := tgbotapi.NewEditMessageText(
		q.Message.Chat.ID,
		q.Message.MessageID,
		text.String(),
	)
	edit.ParseMode = "Markdown"
	edit.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}

	h.api.Send(edit)
}

func (h *Handler) deleteAlias(ctx context.Context, q *tgbotapi.CallbackQuery, aliasID int64) {
	ownerID, err := h.users.GetUserIDByTelegramID(ctx, q.From.ID)
	if err != nil {
		return
	}

	err = h.contacts.DeleteAliasByID(ctx, ownerID, aliasID)
	if err != nil {
		return
	}

	// просто перерисовываем назад
	h.editContactsMenu(ctx, q)
}
