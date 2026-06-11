package kiwifypayload

type Trigger string

const (
	TriggerBilletCreated        Trigger = "billet_created"
	TriggerPixCreated           Trigger = "pix_created"
	TriggerOrderRejected        Trigger = "order_rejected"
	TriggerAbandonedCart        Trigger = "abandoned_cart"
	TriggerOrderApproved        Trigger = "order_approved"
	TriggerSubscriptionRenewed  Trigger = "subscription_renewed"
	TriggerSubscriptionLate     Trigger = "subscription_late"
	TriggerSubscriptionCanceled Trigger = "subscription_canceled"
	TriggerOrderRefunded        Trigger = "order_refunded"
	TriggerChargeback           Trigger = "chargeback"
	TriggerUnknown              Trigger = ""
)

func Classify(p Payload) Trigger {
	if p.WebhookEventType != "" {
		return Trigger(p.WebhookEventType)
	}
	if p.AbandonedID != "" || p.AbandonedStatus == "abandoned" {
		return TriggerAbandonedCart
	}
	return TriggerUnknown
}
