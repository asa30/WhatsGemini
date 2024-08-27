package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"

	"bytes"
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

type OptionalPromptInputs struct {
	Image    []byte
	Video    []byte
	Audio    []byte
	FileData []byte
}

// Function to send POST request to the API
func promptGEMINI(prompt string, options *OptionalPromptInputs) (string, error) {
	var promptInputData []byte
	var resp *genai.GenerateContentResponse
	var err error

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

	prompt_type := "text"

	// Checks if there are additional prompt inputs
	if options != nil {
		// If there was an Image attached, include it in the prompt to Gemini
		if options.Image != nil {
			promptInputData = options.Image
			prompt_type = "image"
		}
		// If there was a Video attached, include it in the prompt to Gemini
		if options.Video != nil {
			promptInputData = options.Video
			prompt_type = "video"
		}
		// If there was an Audio attached, include it in the prompt to Gemini
		if options.Audio != nil {
			promptInputData = options.Audio
			prompt_type = "audio"
		}
		// If there was a File attached, include it in the prompt to Gemini
		if options.FileData != nil {
			promptInputData = options.FileData
			prompt_type = "file"
		}
	}

	// Generate content
	if prompt_type == "text" {
		resp, err = model.GenerateContent(ctx, genai.Text(prompt))
		if err != nil {
			log.Fatal("Error generating content:", err)
		}
	} else {
		// Upload the image to Gemini
		opts := genai.UploadFileOptions{}
		// 
		upload, err := client.UploadFile(ctx, "", bytes.NewReader(promptInputData), &opts)
		if err != nil {
			log.Fatal("Error uploading prompt Image:", err)
		}
		// Construct the prompt
		prompt := []genai.Part{
			genai.FileData{URI: upload.URI},
			genai.Text(prompt),
		}
		// Generate content
		resp, err = model.GenerateContent(ctx, prompt...)
		if err != nil {
			log.Fatal("Error generating content:", err)
		}
	}

	fmt.Println("THE PROMPT")
	fmt.Println(prompt)
	fmt.Println("THE RESPONSE")
	fmt.Println(resp.Candidates[0].Content.Parts)

	outResponse := ""

	for _, part := range resp.Candidates[0].Content.Parts {
		// append text
		outResponse += fmt.Sprintf("%v\n", part)
	}
	return strings.TrimSpace(outResponse), err
}

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

				respMessage, err := promptGEMINI(message, nil)
				if err != nil {
					fmt.Println("Failed to send post request:", err)
				}

				client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					Conversation: proto.String(respMessage),
				})
			}
			// 
			var messageImage = v.Message.GetImageMessage()
			var caption = messageImage.GetCaption()
			if caption == trigger || strings.Contains(caption, trigger) {
				var chatMsg = strings.ReplaceAll(caption, trigger, "")
				userDetail := v.Info.Sender.User

				fmt.Println("The user name is:", userDetail)
				message := chatMsg

				var ImageData []byte = nil
				var err error
				if messageImage != nil {
					fmt.Println("Image attached")
					ImageData, err = client.Download(messageImage)
					if err != nil {
						fmt.Println("Failed to download image:", err)
					}
				}

				attachments := OptionalPromptInputs{
					Image: ImageData,
				}

				respMessage, err := promptGEMINI(message, &attachments)
				if err != nil {
					fmt.Println("Failed to send post request:", err)
				}

				client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					Conversation: proto.String(respMessage),
				})
			}
			// 
			var messageVideo = v.Message.GetVideoMessage()
			caption = messageVideo.GetCaption()
			if caption == trigger || strings.Contains(caption, trigger) {
				var chatMsg = strings.ReplaceAll(caption, trigger, "")
				userDetail := v.Info.Sender.User

				fmt.Println("The user name is:", userDetail)
				message := chatMsg

				var VideoData []byte = nil
				var err error
				if messageVideo != nil {
					fmt.Println("Video attached")
					VideoData, err = client.Download(messageVideo)
					if err != nil {
						fmt.Println("Failed to download video:", err)
					}
				}

				attachments := OptionalPromptInputs{
					Video: VideoData,
				}

				respMessage, err := promptGEMINI(message, &attachments)
				if err != nil {
					fmt.Println("Failed to send post request:", err)
				}

				client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
					Conversation: proto.String(respMessage),
				})
			}
			// 
			var messageDocument = v.Message.GetDocumentMessage()
			caption = messageDocument.GetCaption()
			if caption == trigger || strings.Contains(caption, trigger) {
				var chatMsg = strings.ReplaceAll(caption, trigger, "")
				userDetail := v.Info.Sender.User

				fmt.Println("The user name is:", userDetail)
				message := chatMsg

				var FileData []byte = nil
				var err error
				if messageDocument != nil {
					fmt.Println("File attached")
					FileData, err = client.Download(messageDocument)
					if err != nil {
						fmt.Println("Failed to download file:", err)
					}
				}

				attachments := OptionalPromptInputs{
					FileData: FileData,
				}

				respMessage, err := promptGEMINI(message, &attachments)
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
