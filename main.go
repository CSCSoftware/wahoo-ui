package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/CSCSoftware/wahoo/db"
	"github.com/CSCSoftware/wahoo/wa"
)

//go:embed web/*
var webFS embed.FS

var (
	store  *db.Store
	client *wa.Client
)

func main() {
	storeDir := flag.String("store-dir", "store", "Directory for SQLite databases")
	addr := flag.String("addr", "localhost:8080", "HTTP server address")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser automatically")
	flag.Parse()

	fmt.Println("wahoo-ui - WhatsApp Web Interface")
	fmt.Printf("Store directory: %s\n", *storeDir)

	// Open databases
	var err error
	store, err = db.NewStore(*storeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open databases: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Create and connect WhatsApp client
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err = wa.NewClient(store, *storeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create WhatsApp client: %v\n", err)
		os.Exit(1)
	}

	// Connect in background
	go func() {
		if err := client.Connect(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "WhatsApp connection error: %v\n", err)
		}
	}()

	// Setup HTTP server
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/chats", handleChats)
	mux.HandleFunc("/api/messages", handleMessages)
	mux.HandleFunc("/api/send", handleSend)
	mux.HandleFunc("/api/contacts", handleContacts)
	mux.HandleFunc("/api/status", handleStatus)

	// Static files
	webContent, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	// Handle shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
		client.Disconnect()
		os.Exit(0)
	}()

	// Open browser
	if !*noBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openBrowser("http://" + *addr)
		}()
	}

	fmt.Printf("Starting server at http://%s\n", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// API Handlers

func handleChats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	chats, err := store.ListChats(db.ListChatsOpts{
		Limit:              limit,
		IncludeLastMessage: true,
		SortBy:             "last_active",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(chats)
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	chatJID := r.URL.Query().Get("chat_jid")
	if chatJID == "" {
		http.Error(w, "chat_jid required", http.StatusBadRequest)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	messages, err := store.ListMessages(db.ListMessagesOpts{
		ChatJID:        &chatJID,
		Limit:          limit,
		IncludeContext: false,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(messages)
}

func handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Recipient string `json:"recipient"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if client == nil || !client.IsConnected() {
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   "WhatsApp not connected",
		})
		return
	}

	success, msg := client.SendMessage(req.Recipient, req.Message)
	json.NewEncoder(w).Encode(map[string]any{
		"success": success,
		"message": msg,
	})
}

func handleContacts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "q (query) required", http.StatusBadRequest)
		return
	}

	contacts, err := store.SearchContacts(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(contacts)
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	connected := client != nil && client.IsConnected()
	json.NewEncoder(w).Encode(map[string]any{
		"connected": connected,
	})
}
