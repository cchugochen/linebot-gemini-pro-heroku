package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

const ImageTemperture = 0.8
const ChatTemperture = 0.3

// GeminiImage: 輸入圖片數據，返回生成的文字描述
func GeminiImage(imgData []byte) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(geminiKey))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-pro-vision") // 選擇生成模型
	value := float32(ImageTemperture)
	model.Temperature = &value
	prompt := []genai.Part{
		genai.ImageData("png", imgData),                                           // 加入圖片數據
		genai.Text("Describe this image with scientific detail, reply in zh-TW:"), // 提示語
	}
	log.Println("Begin processing image...")
	resp, err := model.GenerateContent(ctx, prompt...) // 生成內容
	log.Println("Finished processing image...", resp)
	if err != nil {
		log.Fatal(err)
		return "", err
	}

	return printResponse(resp), nil //輸出結果 返回
}

// startNewChatSession	: Start a new chat session
func startNewChatSession() *genai.ChatSession {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(geminiKey))
	if err != nil {
		log.Fatal(err)
	}
	model := client.GenerativeModel("gemini-pro") // 選擇聊天模型
	value := float32(ChatTemperture)
	model.Temperature = &value
	cs := model.StartChat() // 啟動聊天
	return cs
}

// send: Send a message to the chat session
func send(cs *genai.ChatSession, msg string) *genai.GenerateContentResponse {
	if cs == nil {
		cs = startNewChatSession() // 如果會話不存在，則啟動新會話
	}

	ctx := context.Background()
	log.Printf("== Me: %s\n== Model:\n", msg)
	res, err := cs.SendMessage(ctx, genai.Text(msg)) // 發送消息
	if err != nil {
		log.Fatal(err)
	}
	return res
}

// Print response
func printResponse(resp *genai.GenerateContentResponse) string {
	var ret string
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			ret = ret + fmt.Sprintf("%v", part)
			log.Println(part)
		}
	}
	return ret
}
