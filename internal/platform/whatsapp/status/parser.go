package status

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func ExtractStatuses(raw []byte) ([]MessageStatus, error) {
	var p statusPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("whatsapp.status: unmarshal: %w", err)
	}

	var result []MessageStatus
	for _, e := range p.Entry {
		for _, c := range e.Changes {
			for _, st := range c.Value.Statuses {
				if st.ID == "" || st.Status == "" {
					continue
				}
				ms := MessageStatus{
					MessageID:   st.ID,
					Status:      st.Status,
					RecipientID: st.RecipientID,
					Timestamp:   st.Timestamp,
				}
				if len(st.Errors) > 0 {
					ms.ErrorCode = strconv.Itoa(st.Errors[0].Code)
					ms.ErrorTitle = st.Errors[0].Title
				}
				result = append(result, ms)
			}
		}
	}
	return result, nil
}
