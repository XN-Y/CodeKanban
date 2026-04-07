package websession

import (
	"encoding/json"
	"strconv"
	"time"
)

const protocolVersion = 1

// Wire payloads intentionally use short keys to reduce websocket overhead.
// Keep business logic on the semantic structs in types.go and only map to/from
// these short keys at the protocol boundary.
type wireCommandFrame struct {
	Version   int             `json:"v"`
	Kind      string          `json:"k"`
	RequestID string          `json:"rid"`
	SessionID string          `json:"sid,omitempty"`
	Operation string          `json:"op"`
	Payload   json.RawMessage `json:"p,omitempty"`
}

type wireFrame struct {
	Version   int        `json:"v"`
	Kind      string     `json:"k"`
	RequestID string     `json:"rid,omitempty"`
	SessionID string     `json:"sid,omitempty"`
	Timestamp int64      `json:"ts"`
	Operation string     `json:"op,omitempty"`
	Payload   any        `json:"p,omitempty"`
	OK        *int       `json:"ok,omitempty"`
	Session   *wireSess  `json:"s,omitempty"`
	History   *wireHist  `json:"h,omitempty"`
	Event     *wireEvent `json:"e,omitempty"`
	Code      string     `json:"code,omitempty"`
	Message   string     `json:"msg,omitempty"`
	Retry     bool       `json:"retry,omitempty"`
}

type wireSess struct {
	ID              string    `json:"id"`
	ProjectID       string    `json:"pid"`
	WorktreeID      *string   `json:"wid,omitempty"`
	OrderIndex      float64   `json:"oi"`
	Agent           string    `json:"ag"`
	Model           string    `json:"md"`
	ReasoningEffort string    `json:"re"`
	WorkflowMode    string    `json:"wm"`
	PermissionLevel string    `json:"pl"`
	Title           string    `json:"ttl"`
	Cwd             string    `json:"cwd"`
	NativeSessionID *string   `json:"nsid,omitempty"`
	Status          string    `json:"st"`
	Unread          bool      `json:"unr"`
	LastUpdated     int64     `json:"lu"`
	LastMessageAt   *int64    `json:"lma,omitempty"`
	Usage           wireUsage `json:"usa"`
	Cost            float64   `json:"cost"`
}

type wireUsage struct {
	InputTokens       int64 `json:"in"`
	CachedInputTokens int64 `json:"cin"`
	OutputTokens      int64 `json:"out"`
}

type wireHist struct {
	Events       []wireEvent `json:"evs"`
	HasMore      bool        `json:"hm"`
	BeforeCursor string      `json:"bc,omitempty"`
	Total        int         `json:"tot"`
}

type wireEvent struct {
	ID        string         `json:"id"`
	Seq       int64          `json:"sq"`
	Type      string         `json:"tp"`
	RunID     string         `json:"rid2,omitempty"`
	ParentID  string         `json:"pid2,omitempty"`
	Timestamp int64          `json:"ts"`
	Payload   map[string]any `json:"p,omitempty"`
}

func newAckFrame(requestID, op, sessionID string, payload any) wireFrame {
	ok := 1
	return wireFrame{
		Version:   protocolVersion,
		Kind:      "ack",
		RequestID: requestID,
		SessionID: sessionID,
		Timestamp: nowUnixMilli(),
		Operation: op,
		Payload:   payload,
		OK:        &ok,
	}
}

func newErrorFrame(requestID, sessionID, code, message string, retry bool) wireFrame {
	return wireFrame{
		Version:   protocolVersion,
		Kind:      "err",
		RequestID: requestID,
		SessionID: sessionID,
		Timestamp: nowUnixMilli(),
		Code:      code,
		Message:   message,
		Retry:     retry,
	}
}

func newSnapshotFrame(sessionID string, snap SessionSnapshot) wireFrame {
	wireHistory := make([]wireEvent, 0, len(snap.History.Events))
	for _, event := range snap.History.Events {
		wireHistory = append(wireHistory, mapWireEvent(event))
	}
	return wireFrame{
		Version:   protocolVersion,
		Kind:      "snap",
		SessionID: sessionID,
		Timestamp: nowUnixMilli(),
		Session:   mapWireSession(snap.Session),
		History: &wireHist{
			Events:       wireHistory,
			HasMore:      snap.History.HasMore,
			BeforeCursor: snap.History.BeforeCursor,
			Total:        snap.History.Total,
		},
	}
}

func newEventFrame(sessionID string, event Event) wireFrame {
	return wireFrame{
		Version:   protocolVersion,
		Kind:      "evt",
		SessionID: sessionID,
		Timestamp: nowUnixMilli(),
		Event:     ptr(mapWireEvent(event)),
	}
}

func mapWireSession(session SessionSummary) *wireSess {
	var lastMessageAt *int64
	if session.LastMessageAt != nil {
		value := session.LastMessageAt.UnixMilli()
		lastMessageAt = &value
	}
	return &wireSess{
		ID:              session.ID,
		ProjectID:       session.ProjectID,
		WorktreeID:      session.WorktreeID,
		OrderIndex:      session.OrderIndex,
		Agent:           string(session.Agent),
		Model:           session.Model,
		ReasoningEffort: string(session.ReasoningEffort),
		WorkflowMode:    string(session.WorkflowMode),
		PermissionLevel: string(session.PermissionLevel),
		Title:           session.Title,
		Cwd:             session.Cwd,
		NativeSessionID: session.NativeSessionID,
		Status:          string(session.Status),
		Unread:          session.HasUnread,
		LastUpdated:     session.UpdatedAt.UnixMilli(),
		LastMessageAt:   lastMessageAt,
		Usage: wireUsage{
			InputTokens:       session.Usage.InputTokens,
			CachedInputTokens: session.Usage.CachedInputTokens,
			OutputTokens:      session.Usage.OutputTokens,
		},
		Cost: session.Usage.Cost,
	}
}

func mapWireEvent(event Event) wireEvent {
	return wireEvent{
		ID:        event.ID,
		Seq:       event.Seq,
		Type:      event.Type,
		RunID:     event.RunID,
		ParentID:  event.ParentID,
		Timestamp: event.Timestamp.UnixMilli(),
		Payload:   event.Payload,
	}
}

func parseBeforeCursor(raw json.RawMessage) (*int64, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var payload struct {
		BeforeCursor string `json:"bc"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.BeforeCursor == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(payload.BeforeCursor, 10, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func historyCursor(events []Event, hasMore bool) string {
	if !hasMore || len(events) == 0 {
		return ""
	}
	return strconv.FormatInt(events[0].Seq, 10)
}

func nowUnixMilli() int64 {
	return time.Now().UnixMilli()
}

func ptr[T any](value T) *T {
	return &value
}
