package bot

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ParsedDebt struct {
	AmountCents int64
	Currency    string
	RawName     string
	DueDate     time.Time
}

var (
	reAmount    = regexp.MustCompile(`(?i)^\s*([0-9]+(?:[.,][0-9]{1,2})?)\s*([$€£]|usd|eur|gbp|руб|руб\.|р|₽)?\s+(.+)$`)
	reDateDMY   = regexp.MustCompile(`(?i)\b(\d{1,2})[.\-/](\d{1,2})[.\-/](\d{4})\b`)
	reDateWords = regexp.MustCompile(`(?i)\b(\d{1,2})\s+([а-яё]+)\s+(\d{4})\b`)
)

func ParseDebtText(text string) (ParsedDebt, error) {
	// Expect: "<amount><currency> <name...> <date...>"
	m := reAmount.FindStringSubmatch(text)
	if m == nil {
		return ParsedDebt{}, errors.New("не понял сумму. Пример: `300$ Антон 12.12.2025`")
	}
	amountStr := strings.ReplaceAll(m[1], ",", ".")
	cur := strings.TrimSpace(strings.ToLower(m[2]))
	rest := strings.TrimSpace(m[3])

	amountCents, err := parseMoneyToCents(amountStr)
	if err != nil {
		return ParsedDebt{}, errors.New("не понял сумму (формат). Пример: 300 или 300.50")
	}
	currency := normalizeCurrency(cur)

	// find date at end (either dd.mm.yyyy or "12 декабря 2025")
	due, name, err := extractDateAndName(rest)
	if err != nil {
		return ParsedDebt{}, err
	}

	return ParsedDebt{
		AmountCents: amountCents,
		Currency:    currency,
		RawName:     strings.TrimSpace(name),
		DueDate:     due,
	}, nil
}

func parseMoneyToCents(s string) (int64, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(f*100 + 0.5), nil
}

func normalizeCurrency(cur string) string {
	switch cur {
	case "$", "usd":
		return "USD"
	case "€", "eur":
		return "EUR"
	case "£", "gbp":
		return "GBP"
	case "₽", "р", "руб", "руб.":
		return "RUB"
	case "":
		// default: USD (можно сделать настройку на пользователя)
		return "USD"
	default:
		return strings.ToUpper(cur)
	}
}

func extractDateAndName(rest string) (time.Time, string, error) {
	// 1) dd.mm.yyyy inside string (often at end)
	if dm := reDateDMY.FindStringSubmatch(rest); dm != nil {
		dd, _ := strconv.Atoi(dm[1])
		mm, _ := strconv.Atoi(dm[2])
		yy, _ := strconv.Atoi(dm[3])
		d := time.Date(yy, time.Month(mm), dd, 0, 0, 0, 0, time.UTC)

		name := strings.TrimSpace(reDateDMY.ReplaceAllString(rest, ""))
		if name == "" {
			return time.Time{}, "", errors.New("не увидел имя. Пример: `300$ Антон 12.12.2025`")
		}
		return d, name, nil
	}

	// 2) "12 декабря 2025"
	if wm := reDateWords.FindStringSubmatch(rest); wm != nil {
		dd, _ := strconv.Atoi(wm[1])
		monthWord := strings.ToLower(wm[2])
		yy, _ := strconv.Atoi(wm[3])

		mm, ok := ruMonthToNumber(monthWord)
		if !ok {
			return time.Time{}, "", fmt.Errorf("не понял месяц: %s", monthWord)
		}
		d := time.Date(yy, time.Month(mm), dd, 0, 0, 0, 0, time.UTC)

		name := strings.TrimSpace(reDateWords.ReplaceAllString(rest, ""))
		if name == "" {
			return time.Time{}, "", errors.New("не увидел имя. Пример: `300$ Антон 12 декабря 2025`")
		}
		return d, name, nil
	}

	return time.Time{}, "", errors.New("не понял дату. Пример: `12.12.2025` или `12 декабря 2025`")
}

func ruMonthToNumber(m string) (int, bool) {
	switch m {
	case "января", "январь":
		return 1, true
	case "февраля", "февраль":
		return 2, true
	case "марта", "март":
		return 3, true
	case "апреля", "апрель":
		return 4, true
	case "мая", "май":
		return 5, true
	case "июня", "июнь":
		return 6, true
	case "июля", "июль":
		return 7, true
	case "августа", "август":
		return 8, true
	case "сентября", "сентябрь":
		return 9, true
	case "октября", "октябрь":
		return 10, true
	case "ноября", "ноябрь":
		return 11, true
	case "декабря", "декабрь":
		return 12, true
	default:
		return 0, false
	}
}
