package staff

import "context"

const maxNames = 50

// Names returns the display name of each staff member in ids. Anyone who
// cannot be found is left out. It is meant for showing who did something to
// other staff, and never returns email addresses.
func (s *Service) Names(ctx context.Context, ids []string) map[string]string {
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		if _, seen := out[id]; seen || len(out) >= maxNames {
			continue
		}
		if u, err := s.d.Repo.Find(ctx, id); err == nil {
			out[id] = u.Name
		}
	}
	return out
}
