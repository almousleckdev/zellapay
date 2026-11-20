package httpapi

import (
	"net/http"

	"github.com/zellapay/zellapay/internal/audit"
	"github.com/zellapay/zellapay/internal/platform/httpx"
)

const activityLimit = 30

// Activity lists what staff and the system did to a merchant, newest first.
// Actors are shown by name when they are staff.
func (h *Handler) Activity(w http.ResponseWriter, r *http.Request) {
	recs, err := h.d.History.List(r.Context(), audit.Filter{
		TargetKind: "merchant", TargetID: r.PathValue("id"), Limit: activityLimit,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	ids := make([]string, 0, len(recs))
	for _, rec := range recs {
		if rec.Actor.Kind == audit.ActorStaff {
			ids = append(ids, rec.Actor.ID)
		}
	}
	names := h.d.Directory.Names(r.Context(), ids)
	out := activityResponse{Data: make([]activityJSON, len(recs))}
	for i, rec := range recs {
		out.Data[i] = activityJSON{
			Action: rec.Action, Actor: actorName(rec.Actor, names), Reason: rec.Reason,
			At: rec.At, Change: rec.Change,
		}
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// actorName is the staff member's name, or what kind of actor it was.
func actorName(a audit.Actor, names map[string]string) string {
	if n, ok := names[a.ID]; ok && a.Kind == audit.ActorStaff {
		return n
	}
	return string(a.Kind)
}
