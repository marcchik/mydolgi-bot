package repo

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Contacts struct{ pool *pgxpool.Pool }
func NewContacts(p *pgxpool.Pool) *Contacts { return &Contacts{pool: p} }

func (r *Contacts) AddContact(ctx context.Context, ownerID, contactID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO contacts(owner_user_id, contact_user_id)
		VALUES($1,$2)
		ON CONFLICT DO NOTHING
	`, ownerID, contactID)
	return err
}

func (r *Contacts) AddAlias(ctx context.Context, ownerID, contactID int64, alias string) error {
	alias = normalize(alias)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO contact_aliases(owner_user_id, contact_user_id, alias)
		VALUES($1,$2,$3)
		ON CONFLICT DO NOTHING
	`, ownerID, contactID, alias)
	return err
}

func (r *Contacts) FindContactByConfirmingName(ctx context.Context, ownerID int64, rawName string) (contactUserID int64, candidates []ContactCandidate, err error) {
	needle := normalize(rawName)
	// 1) exact alias
	rows, err := r.pool.Query(ctx, `
		SELECT ca.contact_user_id,
		       COALESCE(u.username,'') AS username,
		       COALESCE(u.first_name,'') AS first_name,
		       COALESCE(u.last_name,'')  AS last_name
		FROM contact_aliases ca
		JOIN users u ON u.id = ca.contact_user_id
		WHERE ca.owner_user_id=$1 AND ca.alias=$2
		LIMIT 5
	`, ownerID, needle)
	if err != nil { return 0, nil, err }
	defer rows.Close()

	var list []ContactCandidate
	for rows.Next() {
		var cid int64
		var un, fn, ln string
		if e := rows.Scan(&cid, &un, &fn, &ln); e != nil { return 0, nil, e }
		list = append(list, ContactCandidate{UserID: cid, Username: un, FirstName: fn, LastName: ln})
	}
	if len(list) == 1 {
		return list[0].UserID, list, nil
	}
	if len(list) > 1 {
		return 0, list, nil
	}

	// 2) fuzzy: alias ILIKE %needle%
	rows2, err := r.pool.Query(ctx, `
		SELECT DISTINCT ca.contact_user_id,
		       COALESCE(u.username,'') AS username,
		       COALESCE(u.first_name,'') AS first_name,
		       COALESCE(u.last_name,'')  AS last_name
		FROM contact_aliases ca
		JOIN users u ON u.id = ca.contact_user_id
		WHERE ca.owner_user_id=$1 AND ca.alias ILIKE '%' || $2 || '%'
		LIMIT 5
	`, ownerID, needle)
	if err != nil { return 0, nil, err }
	defer rows2.Close()

	list = nil
	for rows2.Next() {
		var cid int64
		var un, fn, ln string
		if e := rows2.Scan(&cid, &un, &fn, &ln); e != nil { return 0, nil, e }
		list = append(list, ContactCandidate{UserID: cid, Username: un, FirstName: fn, LastName: ln})
	}
	if len(list) == 1 {
		return list[0].UserID, list, nil
	}
	return 0, list, nil
}

type ContactCandidate struct {
	UserID    int64
	Username  string
	FirstName string
	LastName  string
}

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
}
