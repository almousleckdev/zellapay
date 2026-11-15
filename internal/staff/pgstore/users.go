package pgstore

import (
	"context"

	"github.com/zellapay/zellapay/internal/identity"
	"github.com/zellapay/zellapay/internal/staff"
	"github.com/zellapay/zellapay/internal/staff/pgstore/sqlcgen"
)

func userFromRow(r sqlcgen.StaffUser) staff.User {
	return staff.User{
		ID: r.ID, Email: r.Email, Name: r.Name, Role: staff.Role(r.Role),
		Status: staff.Status(r.Status), PasswordHash: r.PasswordHash,
		PasswordChangedAt: r.PasswordChangedAt.Time, CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt.Time,
	}
}

// CreateUser stores a staff account and its audit entries.
func (s *Store) CreateUser(ctx context.Context, u staff.User, fx staff.Effects) error {
	err := s.inTx(ctx, fx, func(q *sqlcgen.Queries) error {
		return q.InsertStaffUser(ctx, sqlcgen.InsertStaffUserParams{
			ID: u.ID, Email: u.Email, Name: u.Name, Role: string(u.Role),
			Status: string(u.Status), PasswordHash: u.PasswordHash,
			PasswordChangedAt: ts(u.PasswordChangedAt), CreatedBy: u.CreatedBy,
			CreatedAt: ts(u.CreatedAt),
		})
	})
	if isUnique(err) {
		return identity.ErrEmailTaken
	}
	return err
}

// FindByEmail loads a member of staff by normalised email.
func (s *Store) FindByEmail(ctx context.Context, email string) (staff.User, error) {
	r, err := sqlcgen.New(s.pool).GetStaffByEmail(ctx, email)
	if err != nil {
		return staff.User{}, notFound(err)
	}
	return userFromRow(r), nil
}

// Find loads a member of staff by ID.
func (s *Store) Find(ctx context.Context, id string) (staff.User, error) {
	r, err := sqlcgen.New(s.pool).GetStaff(ctx, id)
	if err != nil {
		return staff.User{}, notFound(err)
	}
	return userFromRow(r), nil
}

// Rehash replaces oldHash with newHash if oldHash is still current.
func (s *Store) Rehash(ctx context.Context, id, oldHash, newHash string) error {
	return sqlcgen.New(s.pool).Rehash(ctx, sqlcgen.RehashParams{
		ID: id, OldHash: oldHash, PasswordHash: newHash,
	})
}
