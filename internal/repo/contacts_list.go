package repo

import (
	"context"
)

type ContactWithAliases struct {
	UserID    int64
	Username  string
	FirstName string
	LastName  string
	Aliases   []string
}

// Если Contacts уже объявлен — оставь только методы.

func (r *Contacts) ListContactsWithAliases(ctx context.Context, ownerID int64, limit int) ([]ContactWithAliases, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
    c.contact_user_id,
    COALESCE(u.username,'') AS username,
    COALESCE(u.first_name,'') AS first_name,
    COALESCE(u.last_name,'')  AS last_name,
    COALESCE(
        array_agg(a.alias ORDER BY LENGTH(a.alias) DESC)
        FILTER (WHERE a.alias IS NOT NULL),
        ARRAY[]::text[]
    ) AS aliases
FROM contacts c
LEFT JOIN users u
       ON u.id = c.contact_user_id
LEFT JOIN contact_aliases a
       ON a.owner_user_id = c.owner_user_id
      AND a.contact_user_id = c.contact_user_id
WHERE c.owner_user_id = $1
GROUP BY c.contact_user_id, u.username, u.first_name, u.last_name
ORDER BY
    COALESCE(NULLIF(u.username,''), CONCAT_WS(' ', u.first_name, u.last_name)) ASC
LIMIT $2;
	`, ownerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ContactWithAliases, 0, 64)
	for rows.Next() {
		var c ContactWithAliases
		if err := rows.Scan(&c.UserID, &c.Username, &c.FirstName, &c.LastName, &c.Aliases); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
