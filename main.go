package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

var sessionStorage = make(map[string][]ChatMessage)
var storageMutex = &sync.Mutex{}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var chatReq ChatRequest
	err := json.Unmarshal(reqBody, &chatReq)
	if err != nil {
		log.Println(err)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		sessionID = uuid.New().String()
		w.Header().Set("X-Session-ID", sessionID)
	}

	storageMutex.Lock()
	messageHistory, ok := sessionStorage[sessionID]
	if ok {
		chatReq.Messages = append(messageHistory, chatReq.Messages...)
	} else {
		sessionStorage[sessionID] = []ChatMessage{}
	}
	storageMutex.Unlock()

	chatReq.Model = "gpt-3.5-turbo"
	apiKey := "sk-xxx"

	chatRes, err := getChatCompletion(apiKey, chatReq, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	storageMutex.Lock()
	sessionStorage[sessionID] = append(sessionStorage[sessionID], chatRes.Choices[0].Message)
	storageMutex.Unlock()

	err = json.NewEncoder(w).Encode(chatRes)
	if err != nil {
		log.Println(err)
		return
	}
}

func getChatCompletion(apiKey string, chatReq ChatRequest, sessionID string) (*ChatResponse, error) {
	jsonData, _ := json.Marshal(chatReq)
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", strings.NewReader(string(jsonData)))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("X-Session-ID", sessionID)

	client := &http.Client{Timeout: time.Second * 30}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("API 请求错误: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		log.Printf("API 响应错误: %v, %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("API 响应错误: %v", resp.StatusCode)
	}

	var chatRes ChatResponse
	err = json.NewDecoder(resp.Body).Decode(&chatRes)
	if err != nil {
		log.Printf("API 响应解码错误: %v", err)
		return nil, err
	}
	return &chatRes, nil
}
func generateSessionID() string {
	return uuid.New().String()
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/chat", chatHandler).Methods("POST")
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("static")))
	http.Handle("/", r)
	log.Printf("Server starting on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
