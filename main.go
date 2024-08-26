package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	_ "github.com/joho/godotenv/autoload"

	// "bytes"
	// "encoding/json"
	// "io"
	// "net/http"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"

	// "errors"
	"log"

	"github.com/google/generative-ai-go/genai"
 	"google.golang.org/api/option"
)

type Prompt string

type Prompt_with_Images struct {
	Prompt Prompt
	Images []byte
}

type Prompt_with_PDF struct {
	Prompt Prompt
	PDF []byte
}

// Function to send POST request to the API
func sendPostRequestGEMINI(prompt string) (string, error) {
	// Get the API key and ModelID from environment variables
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY environment variable not set")
	}
	// 
	model_id := os.Getenv("GEMINI_MODEL_ID")
	if model_id == "" {
		return "", fmt.Errorf("GEMINI_MODEL_ID environment variable not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal("Error creating client:", err)
	}

	model := client.GenerativeModel(model_id)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
    	log.Fatal("Error generating content:", err)
	}

	fmt.Println("THE FUCKING PROMPT")
	fmt.Println(prompt)
	fmt.Println("THE FUCKING RESPONSE")

	fmt.Println(resp.Candidates[0].Content.Parts)
	
	outResponse := ""

	for _, part := range resp.Candidates[0].Content.Parts {
		// append text 
		outResponse += fmt.Sprintf("%v\n", part)
	}
	return strings.TrimSpace(outResponse), err
}

// func htmlToWhatsAppFormat(html string) string {
// 	// Replace HTML tags with WhatsApp-friendly formatting
// 	html = strings.ReplaceAll(html, "</p>\n", "\n")
// 	html = strings.ReplaceAll(html, "</p>", "\n")
// 	html = strings.ReplaceAll(html, "<p>", "")
// 	html = strings.ReplaceAll(html, "<ol>", "- ")
// 	html = strings.ReplaceAll(html, "</ol>", "\n")
// 	html = strings.ReplaceAll(html, "<li>", "- ")
// 	html = strings.ReplaceAll(html, "</li>", "\n")
// 	html = strings.ReplaceAll(html, "<br>", "\n")

// 	return html
// }


func GetEventHandler(client *whatsmeow.Client) func(interface{}) {
	// declare the trigger
	trigger := os.Getenv("TRIGGER")
	if trigger == "" {
		trigger = "0>"
		fmt.Println("TRIGGER environment variable not set. Using default trigger: 0>")
	}
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			var messageBody = v.Message.GetConversation()
			if messageBody == trigger || strings.Contains(messageBody, trigger) {
				var chatMsg = strings.ReplaceAll(messageBody, trigger, "")
				userDetail := v.Info.Sender.User

				fmt.Println("The user name is:", userDetail)
				message := chatMsg

				respMessage, err := sendPostRequestGEMINI(message)
				if err != nil {
					fmt.Println("Failed to send post request:", err)
				}

				client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					Conversation: proto.String(respMessage),
				})
			}
		}
	}
}

func main() {

	dbLog := waLog.Stdout("Database", "DEBUG", true)
	// Make sure you add appropriate DB connector imports, e.g. github.com/mattn/go-sqlite3 for SQLite as we did in this minimal working example
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.AddEventHandler(GetEventHandler(client))

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code here
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				// or just manually `echo 2@... | qrencode -t ansiutf8` in a terminal:
				// fmt.Println("QR code:", evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
