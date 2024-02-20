// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/line/line-bot-sdk-go/v8/linebot"
	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"
	"github.com/line/line-bot-sdk-go/v8/linebot/webhook"
)

// 定義全局變量
var bot *messaging_api.MessagingApiAPI      // LINE消息API客戶端
var blob *messaging_api.MessagingApiBlobAPI // 處理大型資料 API 客戶端
var geminiKey string                        // Gemini API 金鑰
var channelToken string                     // LINE 頻道令牌

// 建立一個 map 來儲存每個用戶的 ChatSession
var userSessions = make(map[string]*genai.ChatSession)

func main() {
	var err error
	geminiKey = os.Getenv("GOOGLE_GEMINI_API_KEY")
	channelToken = os.Getenv("ChannelAccessToken")
	bot, err = messaging_api.NewMessagingApiAPI(channelToken) // 初始化 LINE消息API客戶端
	if err != nil {
		log.Fatal(err) // 如果初始化失敗，則終止程式
	}

	blob, err = messaging_api.NewMessagingApiBlobAPI(channelToken) // 初始化處理大型資料 API 客戶端
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/callback", callbackHandler) // 設定回調處理器
	port := os.Getenv("PORT")                     // 從環境變數獲取服務器端口
	addr := fmt.Sprintf(":%s", port)              // 格式化服務器地址
	http.ListenAndServe(addr, nil)                // 啟動 HTTP 服務器
}

// 回覆文本消息
func replyText(replyToken, text string, firstTime bool) error {
	// 如果是第一次對話，添加初始化提示语
	if firstTime {
		text = "You are a helpful assistant with precise and logical thinking. " + text
	}

	if _, err := bot.ReplyMessage(
		&messaging_api.ReplyMessageRequest{
			ReplyToken: replyToken, // 回覆令牌
			Messages: []messaging_api.MessageInterface{
				&messaging_api.TextMessage{
					Text: text, // 要回覆的文本內容
				},
			},
		},
	); err != nil {
		return err // 回覆失敗時返回錯誤
	}
	return nil // 回覆成功
}

// 處理LINE平台的回調事件
func callbackHandler(w http.ResponseWriter, r *http.Request) {
	cb, err := webhook.ParseRequest(os.Getenv("ChannelSecret"), r) // 解析請求
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(400) // 簽名無效時回傳狀態碼
		} else {
			w.WriteHeader(500) // 簽名無效時回傳狀態碼
		}
		return
	}

	// 遍歷所有事件
	for _, event := range cb.Events {
		log.Printf("Got event %v", event) // 記錄事件資訊

		// 根據事件類型進行分支處理
		switch e := event.(type) {
		case webhook.MessageEvent: // 處理消息事件
			switch message := e.Message.(type) {

			// 處理文字型訊息
			case webhook.TextMessageContent:
				req := message.Text

				// 檢查訊息: 如果不是以 "@#" 開頭，則不進行任何處理
				if !strings.HasPrefix(req, "@#") {

					return
				}

				// 移除 "@#" 前綴，以便處理餘下的訊息
				req = strings.TrimPrefix(req, "@#")

				var uID string // 取得用戶或群組/聊天室 ID
				switch source := e.Source.(type) {
				case *webhook.UserSource:
					uID = source.UserId
				case *webhook.GroupSource:
					uID = source.UserId
				case *webhook.RoomSource:
					uID = source.UserId
				}

				// 檢查是否已經有這個用戶的 ChatSession
				cs, ok := userSessions[uID]
				if !ok {
					// 如果沒有，則創建一個新的 ChatSession
					cs = startNewChatSession()
					userSessions[uID] = cs
				}
				if strings.EqualFold(req, "reset") {
					// 如果用戶輸入 "reset"，重置記憶，創建一個新的 ChatSession
					firstTime := !ok // 如果ok为false，说明用户会话是新的，firstTime为true
					cs = startNewChatSession()
					userSessions[uID] = cs
					if err := replyText(e.ReplyToken, "很高興初次見到你，我是Gemini，請問有什麼想了解的嗎？", firstTime); err != nil {
						log.Print(err)
					}
					return
				}

				// 使用既有 ChatSession 來處理文字訊息 & Reply with Gemini result
				res := send(cs, req)
				ret := printResponse(res)
				// 在调用replyText时，检查是否为新会话
				firstTime := !ok // 如果ok为false，说明用户会话是新的，firstTime为true
				if err := replyText(e.ReplyToken, ret, firstTime); err != nil {
					log.Print(err)
				}

			// 處理貼圖消息
			case webhook.StickerMessageContent:
				var kw string
				for _, k := range message.Keywords {
					kw = kw + "," + k
				}

				outStickerResult := fmt.Sprintf("收到貼圖訊息: %s, pkg: %s kw: %s  text: %s", message.StickerId, message.PackageId, kw, message.Text)
				if err := replyText(e.ReplyToken, outStickerResult); err != nil {
					log.Print(err)
				}

			// 處理image圖片消息
			case webhook.ImageMessageContent:
				log.Println("收到圖片類訊息 ID:", message.Id)

				//Get image binary from LINE server based on message ID.
				content, err := blob.GetMessageContent(message.Id)
				if err != nil {
					log.Println("無法取得圖片的資訊:", err)
				}
				defer content.Body.Close()
				data, err := io.ReadAll(content.Body)
				if err != nil {
					log.Fatal(err)
				}
				ret, err := GeminiImage(data)
				if err != nil {
					ret = "無法辨識圖片內容，請重新輸入:" + err.Error()
				}
				if err := replyText(e.ReplyToken, ret); err != nil {
					log.Print(err)
				}

			// Handle only video message
			case webhook.VideoMessageContent:
				log.Println("收到影片類訊息 ID:", message.Id)

			default:
				log.Printf("無法處理影片類訊息: %v", message)
			}
		case webhook.FollowEvent:
			log.Printf("message: Got followed event")
		case webhook.PostbackEvent:
			data := e.Postback.Data
			log.Printf("Unknown message: Got postback: " + data)
		case webhook.BeaconEvent:
			log.Printf("Got beacon: " + e.Beacon.Hwid)
		}
	}
}
