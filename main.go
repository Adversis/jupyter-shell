package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Message represents a Jupyter terminal WebSocket message
type Message []interface{}

// JupyterClient handles the connection to Jupyter
type JupyterClient struct {
	baseURL   string
	token     string
	terminalID string
	conn      *websocket.Conn
}

// NewJupyterClient creates a new client
func NewJupyterClient(baseURL, token string) *JupyterClient {
	return &JupyterClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
	}
}

// CreateTerminal creates a new terminal session
func (c *JupyterClient) CreateTerminal() error {
	url := fmt.Sprintf("%s/api/terminals", c.baseURL)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	
	// Add token if provided
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", c.token))
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create terminal: %s - %s", resp.Status, body)
	}
	
	// Parse response to get terminal ID
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	
	name, ok := result["name"].(string)
	if !ok {
		return fmt.Errorf("terminal ID not found in response")
	}
	
	c.terminalID = name
	fmt.Printf("Created terminal: %s\n", c.terminalID)
	return nil
}

// Connect establishes WebSocket connection to the terminal
func (c *JupyterClient) Connect() error {
	// Convert HTTP URL to WebSocket URL
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	
	u.Path = fmt.Sprintf("/terminals/websocket/%s", c.terminalID)
	
	// Add token to query parameters if provided
	if c.token != "" {
		q := u.Query()
		q.Set("token", c.token)
		u.RawQuery = q.Encode()
	}
	
	fmt.Printf("Connecting to: %s\n", u.String())
	
	// Create WebSocket connection
	dialer := websocket.DefaultDialer
	header := http.Header{}
	
	conn, _, err := dialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("websocket dial error: %v", err)
	}
	
	c.conn = conn
	return nil
}

// ReadMessages handles incoming messages from the terminal
func (c *JupyterClient) ReadMessages() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}
		
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("failed to parse message: %v", err)
			continue
		}
		
		if len(msg) < 2 {
			continue
		}
		
		msgType, ok := msg[0].(string)
		if !ok {
			continue
		}
		
		switch msgType {
		case "stdout":
			if output, ok := msg[1].(string); ok {
				fmt.Print(output)
			}
		case "setup":
			log.Println("Terminal ready")
		case "disconnect":
			log.Println("Terminal disconnected")
			return
		}
	}
}

// SendCommand sends a command to the terminal
func (c *JupyterClient) SendCommand(cmd string) error {
	// Ensure command ends with newline
	if !strings.HasSuffix(cmd, "\n") {
		cmd += "\n"
	}
	
	msg := Message{"stdin", cmd}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Close cleanly closes the connection
func (c *JupyterClient) Close() error {
	if c.conn != nil {
		// Send exit command
		c.SendCommand("exit")
		time.Sleep(500 * time.Millisecond)
		
		// Close WebSocket
		return c.conn.Close()
	}
	return nil
}

// InteractiveShell provides an interactive terminal experience
func (c *JupyterClient) InteractiveShell() {
	scanner := bufio.NewScanner(os.Stdin)
	
	// Give terminal time to initialize
	time.Sleep(1 * time.Second)
	
	fmt.Println("\nJupyter Terminal Shell")
	fmt.Println("Type 'exit' or press Ctrl+C to quit")
	fmt.Println("----------------------------------------")
	
	for {
		// Show prompt (this is a bit hacky since we don't track the real prompt)
		fmt.Print("\n$ ")
		
		if !scanner.Scan() {
			break
		}
		
		cmd := scanner.Text()
		
		// Handle exit
		if cmd == "exit" {
			break
		}
		
		// Send command
		if err := c.SendCommand(cmd); err != nil {
			log.Printf("failed to send command: %v", err)
			break
		}
		
		// Give terminal time to process and respond
		time.Sleep(100 * time.Millisecond)
	}
}

func main() {
	var (
		serverURL = flag.String("url", "http://localhost:8888", "Jupyter server URL")
		token     = flag.String("token", "", "Jupyter authentication token (optional)")
		termID    = flag.String("term", "", "Existing terminal ID (optional)")
	)
	flag.Parse()
	
	// Create client
	client := NewJupyterClient(*serverURL, *token)
	
	// Create or use existing terminal
	if *termID != "" {
		client.terminalID = *termID
		fmt.Printf("Using existing terminal: %s\n", client.terminalID)
	} else {
		if err := client.CreateTerminal(); err != nil {
			log.Fatalf("Failed to create terminal: %v", err)
		}
	}
	
	// Connect to WebSocket
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	
	// Start reading messages in background
	go client.ReadMessages()
	
	// Interactive shell or single command mode
	if flag.NArg() > 0 {
		// Single command mode
		cmd := strings.Join(flag.Args(), " ")
		if err := client.SendCommand(cmd); err != nil {
			log.Fatalf("Failed to send command: %v", err)
		}
		// Wait for output
		time.Sleep(2 * time.Second)
	} else {
		// Interactive mode
		client.InteractiveShell()
	}
	
	fmt.Println("\nGoodbye!")
}
