package pgstore

import (
	"github.com/zellapay/zellapay/internal/webhooks"
	"github.com/zellapay/zellapay/internal/webhooks/pgstore/sqlcgen"
)

// eventFrom converts a stored event row.
func eventFrom(r sqlcgen.WebhookEvent) webhooks.Event {
	return webhooks.Event{
		ID: r.ID, MerchantID: r.MerchantID, Type: r.Type, ObjectID: r.ObjectID,
		Payload: r.Payload, CreatedAt: r.CreatedAt.Time,
	}
}

func workFrom(r sqlcgen.GetDeliveryWorkRow) webhooks.Work {
	return webhooks.Work{
		Delivery: webhooks.Delivery{
			ID: r.ID, EventID: r.EventID, EndpointID: r.EndpointID,
			Status: webhooks.DeliveryStatus(r.Status), Attempts: int(r.Attempts),
			NextAttemptAt: timePtr(r.NextAttemptAt), RetryUntil: r.RetryUntil.Time,
			LastStatus: int(r.LastStatus), LastError: r.LastError,
			CreatedAt: r.CreatedAt.Time, CompletedAt: timePtr(r.CompletedAt),
		},
		URL:          r.Url,
		SecretSealed: r.SecretEnc,
		Event: webhooks.Event{
			ID: r.EventID, MerchantID: r.MerchantID, Type: r.Type, ObjectID: r.ObjectID,
			Payload: r.Payload, CreatedAt: r.EventCreatedAt.Time,
		},
	}
}
