package terminal

import (
	"sort"
	"sync"
	"time"

	"code-kanban/utils/ai_assistant2"
)

// CompletionRecord 代表一个AI执行完成的记录
type CompletionRecord struct {
	ID          string                         `json:"id"`
	SessionID   string                         `json:"sessionId"`
	ProjectID   string                         `json:"projectId"`
	ProjectName string                         `json:"projectName,omitempty"`
	Title       string                         `json:"title"`
	Assistant   *ai_assistant2.AIAssistantInfo `json:"assistant"`
	CompletedAt time.Time                      `json:"completedAt"`
	// State 表示当前卡片状态，working 时仍保留卡片
	State string `json:"state,omitempty"`
	// LastUserInput 存储用户上次输入的信息
	LastUserInput string `json:"lastUserInput,omitempty"`
	// Dismissed 标记用户是否已主动关闭此通知
	Dismissed bool `json:"dismissed"`
}

// ApprovalRecord 代表一个等待审批的记录
type ApprovalRecord struct {
	ID          string                         `json:"id"`
	SessionID   string                         `json:"sessionId"`
	ProjectID   string                         `json:"projectId"`
	ProjectName string                         `json:"projectName,omitempty"`
	Title       string                         `json:"title"`
	Assistant   *ai_assistant2.AIAssistantInfo `json:"assistant"`
	RequestedAt time.Time                      `json:"requestedAt"`
	// Dismissed 标记用户是否已主动关闭此通知
	Dismissed bool `json:"dismissed"`
}

// RecordManager 管理完成记录和审批记录
// 使用 sync.Map 实现并发安全，每个 session 只保留一条记录
type RecordManager struct {
	// completions 存储完成记录，key 为 sessionId
	completions sync.Map // sessionId -> *CompletionRecord
	// approvals 存储审批记录，key 为 sessionId
	approvals sync.Map // sessionId -> *ApprovalRecord
}

// NewRecordManager 创建新的记录管理器
func NewRecordManager() *RecordManager {
	return &RecordManager{}
}

// AddCompletion 添加一个完成记录，每个 session 只保留最新的一条
func (rm *RecordManager) AddCompletion(record *CompletionRecord) {
	if record.State == "" {
		record.State = "completed"
	}
	rm.completions.Store(record.SessionID, record)
}

// AddApproval 添加一个审批记录，每个 session 只保留最新的一条
func (rm *RecordManager) AddApproval(record *ApprovalRecord) {
	rm.approvals.Store(record.SessionID, record)
}

// GetCompletions 获取所有未关闭的完成记录，按时间降序排列
func (rm *RecordManager) GetCompletions() []*CompletionRecord {
	result := make([]*CompletionRecord, 0)
	rm.completions.Range(func(_, value any) bool {
		if record, ok := value.(*CompletionRecord); ok && !record.Dismissed {
			result = append(result, record)
		}
		return true
	})
	// 按 CompletedAt 降序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].CompletedAt.After(result[j].CompletedAt)
	})
	return result
}

// GetApprovals 获取所有未关闭的审批记录，按时间降序排列
func (rm *RecordManager) GetApprovals() []*ApprovalRecord {
	result := make([]*ApprovalRecord, 0)
	rm.approvals.Range(func(_, value any) bool {
		if record, ok := value.(*ApprovalRecord); ok && !record.Dismissed {
			result = append(result, record)
		}
		return true
	})
	// 按 RequestedAt 降序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].RequestedAt.After(result[j].RequestedAt)
	})
	return result
}

// DismissCompletion 关闭一个完成记录
func (rm *RecordManager) DismissCompletion(recordID string) bool {
	found := false
	rm.completions.Range(func(_, value any) bool {
		if record, ok := value.(*CompletionRecord); ok && record.ID == recordID {
			record.Dismissed = true
			found = true
			return false // 停止遍历
		}
		return true
	})
	return found
}

// DismissApproval 关闭一个审批记录
func (rm *RecordManager) DismissApproval(recordID string) bool {
	found := false
	rm.approvals.Range(func(_, value any) bool {
		if record, ok := value.(*ApprovalRecord); ok && record.ID == recordID {
			record.Dismissed = true
			found = true
			return false // 停止遍历
		}
		return true
	})
	return found
}

// ClearSessionRecords 清除某个 session 的所有记录（当 session 关闭或状态变化时）
func (rm *RecordManager) ClearSessionRecords(sessionID string) {
	rm.completions.Delete(sessionID)
	rm.approvals.Delete(sessionID)
}

// ClearCompletionsBySession 清除某个 session 的完成记录
func (rm *RecordManager) ClearCompletionsBySession(sessionID string) {
	rm.completions.Delete(sessionID)
}

// ClearApprovalsBySession 清除某个 session 的审批记录（当状态从 waiting_approval 变化时）
func (rm *RecordManager) ClearApprovalsBySession(sessionID string) {
	rm.approvals.Delete(sessionID)
}

// UpdateCompletionStateBySession 更新 session 对应的完成记录状态（例如切回 working）
// 同时更新 CompletedAt 时间戳，使记录在排序时提升到最顶部
func (rm *RecordManager) UpdateCompletionStateBySession(sessionID string, state string) bool {
	if value, ok := rm.completions.Load(sessionID); ok {
		if record, ok := value.(*CompletionRecord); ok {
			record.State = state
			record.CompletedAt = time.Now() // 每次状态更新都刷新时间戳
			return true
		}
	}
	return false
}

// UpdateCompletionBySession 更新 session 对应的完成记录状态和用户输入
// 如果 userInput 非空，则同时更新 LastUserInput 字段
// 同时更新 CompletedAt 时间戳，使记录在排序时提升到最顶部
func (rm *RecordManager) UpdateCompletionBySession(sessionID string, state string, userInput string) bool {
	if value, ok := rm.completions.Load(sessionID); ok {
		if record, ok := value.(*CompletionRecord); ok {
			record.State = state
			record.CompletedAt = time.Now() // 每次状态更新都刷新时间戳
			if userInput != "" {
				record.LastUserInput = userInput
			}
			return true
		}
	}
	return false
}

// GetCompletion 获取单个完成记录
func (rm *RecordManager) GetCompletion(recordID string) *CompletionRecord {
	var found *CompletionRecord
	rm.completions.Range(func(_, value any) bool {
		if record, ok := value.(*CompletionRecord); ok && record.ID == recordID {
			found = record
			return false
		}
		return true
	})
	return found
}

// GetApproval 获取单个审批记录
func (rm *RecordManager) GetApproval(recordID string) *ApprovalRecord {
	var found *ApprovalRecord
	rm.approvals.Range(func(_, value any) bool {
		if record, ok := value.(*ApprovalRecord); ok && record.ID == recordID {
			found = record
			return false
		}
		return true
	})
	return found
}
