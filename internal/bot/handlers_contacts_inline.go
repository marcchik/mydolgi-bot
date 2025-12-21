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
