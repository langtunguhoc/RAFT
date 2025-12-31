package raft

import (
	"encoding/json"
	"fmt"
	"os"
)

// Cấu trúc file JSON (Giống hệt hình bạn gửi)
type StorageState struct {
	CurrentTerm int64
	VotedFor    string
	Log         []LogEntry
}

// Hàm lưu trạng thái ra file
func (n *Node) persist() {
	// 1. Gom dữ liệu cần lưu
	state := StorageState{
		CurrentTerm: n.currentTerm,
		VotedFor:    n.votedFor,
		Log:         n.log,
	}

	// 2. Chuyển thành JSON đẹp
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	// 3. Ghi vào file (Ví dụ: logs/node1_storage.json)
	filename := fmt.Sprintf("logs/%s_storage.json", n.id)
	
	// Ghi đè file cũ (0644 là quyền đọc ghi cơ bản)
	_ = os.WriteFile(filename, data, 0644)
}