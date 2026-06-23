package pendingexpense

import "encoding/json"

type Draft struct {
	AmountCents   int64
	Merchant      string
	PaymentMethod string
	Direction     string
	OccurredAt    string
	CategoryID    string
	CategoryPath  string
}

func Encode(d Draft) ([]byte, error) {
	return json.Marshal(d)
}

func Decode(raw []byte) (Draft, error) {
	var d Draft
	if err := json.Unmarshal(raw, &d); err != nil {
		return Draft{}, err
	}
	return d, nil
}
