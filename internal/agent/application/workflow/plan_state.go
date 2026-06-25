package workflow

type PlanStepSerialized struct {
	Index            int     `json:"index"`
	IntentKind       string  `json:"intent_kind"`
	AmountCents      int64   `json:"amount_cents"`
	Merchant         string  `json:"merchant"`
	CategoryHint     string  `json:"category_hint"`
	PaymentMethod    string  `json:"payment_method"`
	CardHint         string  `json:"card_hint"`
	CategoryName     string  `json:"category_name"`
	GoalName         string  `json:"goal_name"`
	RefMonth         string  `json:"ref_month"`
	RawText          string  `json:"raw_text"`
	Installments     int     `json:"installments"`
	Direction        string  `json:"direction"`
	Frequency        string  `json:"frequency"`
	DayOfMonth       int     `json:"day_of_month"`
	ClosingDay       int     `json:"closing_day"`
	DueDay           int     `json:"due_day"`
	LimitCents       int64   `json:"limit_cents"`
	Percentage       int     `json:"percentage"`
	NewNickname      string  `json:"new_nickname"`
	NewName          string  `json:"new_name"`
	NewClosingDay    int     `json:"new_closing_day"`
	NewDueDay        int     `json:"new_due_day"`
	CardName         string  `json:"card_name"`
	Nickname         string  `json:"nickname"`
	Confidence       float64 `json:"confidence"`
	Months           int     `json:"months"`
	SourceCompetence string  `json:"source_competence"`
}

type PlanState struct {
	UserID       string               `json:"user_id"`
	Channel      string               `json:"channel"`
	MessageID    string               `json:"message_id"`
	Text         string               `json:"text"`
	LLMModel     string               `json:"llm_model"`
	PromptSHA256 string               `json:"prompt_sha256"`
	DirectReply  string               `json:"direct_reply"`
	RawResponse  string               `json:"raw_response"`
	Steps        []PlanStepSerialized `json:"steps"`
	Cursor       int                  `json:"cursor"`
	Replies      []string             `json:"replies"`
	Failed       bool                 `json:"failed"`
	ResumeText   string               `json:"resume_text"`
}
