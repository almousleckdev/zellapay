package pgstore

import (
	"context"
	"net/netip"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zellapay/zellapay/internal/merchants"
	"github.com/zellapay/zellapay/internal/merchants/pgstore/sqlcgen"
)

// keyRow is the column set shared by every key query.
type keyRow struct {
	id, merchantID, lookupID  string
	secretHash                []byte
	mode                      string
	scopes                    []string
	allow                     []netip.Prefix
	created, expires, revoked pgtype.Timestamptz
	label                     string
}

func keyFromRow(r keyRow) merchants.APIKey {
	return merchants.APIKey{
		ID:          r.id,
		MerchantID:  r.merchantID,
		LookupID:    r.lookupID,
		SecretHash:  r.secretHash,
		Mode:        merchants.Mode(r.mode),
		Scopes:      scopesFromStrings(r.scopes),
		IPAllowlist: r.allow,
		CreatedAt:   r.created.Time,
		ExpiresAt:   timePtr(r.expires),
		RevokedAt:   timePtr(r.revoked),
		Label:       r.label,
	}
}

// ListKeys returns a merchant's keys, without secret hashes.
func (s *Store) ListKeys(ctx context.Context, merchantID string) ([]merchants.APIKey, error) {
	rows, err := sqlcgen.New(s.pool).ListAPIKeys(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	keys := make([]merchants.APIKey, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, keyFromRow(keyRow{
			id: r.ID, merchantID: r.MerchantID, lookupID: r.LookupID,
			mode: r.Mode, scopes: r.Scopes, allow: r.IpAllowlist,
			created: r.CreatedAt, expires: r.ExpiresAt, revoked: r.RevokedAt, label: r.Label,
		}))
	}
	return keys, nil
}

func scopeStrings(in []merchants.Scope) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

func scopesFromStrings(in []string) []merchants.Scope {
	out := make([]merchants.Scope, len(in))
	for i, s := range in {
		out[i] = merchants.Scope(s)
	}
	return out
}

func nonNil(p []netip.Prefix) []netip.Prefix {
	if p == nil {
		return []netip.Prefix{}
	}
	return p
}
