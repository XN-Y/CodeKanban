package websession

import "time"

func cloneToolRequestQuestions(questions []toolRequestQuestion) []toolRequestQuestion {
	if len(questions) == 0 {
		return nil
	}
	cloned := make([]toolRequestQuestion, 0, len(questions))
	for _, question := range questions {
		nextQuestion := question
		if len(question.Options) > 0 {
			nextQuestion.Options = append([]toolRequestOption(nil), question.Options...)
		}
		cloned = append(cloned, nextQuestion)
	}
	return cloned
}

func clonePendingUserInput(input *PendingUserInput) *PendingUserInput {
	if input == nil {
		return nil
	}
	cloned := &PendingUserInput{
		ItemID:    input.ItemID,
		Prompt:    input.Prompt,
		Questions: cloneToolRequestQuestions(input.Questions),
	}
	if input.RequestedAt != nil {
		requestedAt := *input.RequestedAt
		cloned.RequestedAt = &requestedAt
	}
	return cloned
}

func pendingUserInputFromHistory(items []HistoryItem) *PendingUserInput {
	var pending *PendingUserInput
	for _, item := range items {
		if item.Detail != nil && item.Detail.Type == "user_input_request" {
			var requestedAt *time.Time
			if item.Timestamp != nil {
				value := *item.Timestamp
				requestedAt = &value
			} else if item.ObservedAt != nil {
				value := *item.ObservedAt
				requestedAt = &value
			}
			itemID := item.ID
			if item.SourceItemID != nil && *item.SourceItemID != "" {
				itemID = *item.SourceItemID
			}
			pending = &PendingUserInput{
				ItemID:      itemID,
				Prompt:      firstNonEmpty(item.Detail.Prompt, item.Text),
				Questions:   cloneToolRequestQuestions(item.Detail.Questions),
				RequestedAt: requestedAt,
			}
			continue
		}
		if (item.Detail != nil && item.Detail.Type == "user_input_response") || item.Kind == "user" {
			pending = nil
			continue
		}
		if item.ItemType == "run_abort" || item.ItemType == "run_fail" {
			pending = nil
		}
	}
	return clonePendingUserInput(pending)
}
