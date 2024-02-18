// 用於在本地環境運行
package main

import (
	"log"
	"os"
	"path/filepath"
)

// RecordMessage 用於記錄消息到指定的檔案
func RecordMessage(userID, roomID, groupID, message string) {
	// 根據存在的 ID 類型建立檔案路徑
	var folderName string
	if userID != "" {
		folderName = "UserID_" + userID
	} else if roomID != "" {
		folderName = "RoomID_" + roomID
	} else if groupID != "" {
		folderName = "GroupID_" + groupID
	}

	// 確保子資料夾存在
	dirPath := filepath.Join("conversations", folderName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.Fatalf("Failed to create directory: %v", err)
	}

	// 建立或打開對應的檔案
	filePath := filepath.Join(dirPath, "messages.txt")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// 寫入消息
	if _, err := file.WriteString(message + "\n"); err != nil {
		log.Fatalf("Failed to write message: %v", err)
	}
}

// 假設這是處理 LINE webhook 事件的函數
func handleMessageEvent(userID, roomID, groupID, messageText string) {
	// 調用 RecordMessage 來記錄消息
	RecordMessage(userID, roomID, groupID, messageText)
}

func main() {
	// 示範調用
	handleMessageEvent("123", "", "", "Hello, world!")
	handleMessageEvent("", "456", "", "Hello from the room!")
	handleMessageEvent("", "", "789", "Hello from the group!")
}
