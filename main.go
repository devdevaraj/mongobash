package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/chzyer/readline"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AppState struct {
	client     *mongo.Client
	currentDB  string
	currentCol string
}

func main() {
	var uri string
	flag.StringVar(&uri, "mongodb", "mongodb://localhost:27017", "MongoDB connection URI")
	flag.Parse()

	// Connect to MongoDB
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	fmt.Printf("Connected to %s\n", uri)

	appState := &AppState{
		client: client,
	}

	appState.runREPL()
}

func (s *AppState) runREPL() {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          fmt.Sprintf("\033[32mmongobash:\033[33m%s\033[0m> ", s.pwd()),
		HistoryFile:     "/tmp/mongobash_history",
		AutoComplete:    s,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		return
	}
	defer rl.Close()

	for {
		rl.SetPrompt(fmt.Sprintf("\033[32mmongobash:\033[33m%s\033[0m> ", s.pwd()))
		line, err := rl.Readline()
		if err != nil { // EOF or Interrupt
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		command := parts[0]
		args := parts[1:]

		switch command {
		case "exit", "quit":
			return
		case "pwd":
			fmt.Println(s.pwd())
		case "ls":
			s.handleLs()
		case "cd":
			if len(args) < 1 {
				fmt.Println("Usage: cd <target>")
			} else {
				s.handleCd(args[0])
			}
		case "cat":
			if len(args) < 1 {
				fmt.Println("Usage: cat <_id>")
			} else {
				s.handleCat(args[0])
			}
		case "rm":
			if len(args) < 1 {
				fmt.Println("Usage: rm <_id>")
			} else {
				s.handleRm(args[0])
			}
		case "clear":
			fmt.Print("\033[H\033[2J")
		case "help":
			fmt.Println("Commands:")
			fmt.Println("  ls         List databases, collections, or documents")
			fmt.Println("  cd <path>  Change directory")
			fmt.Println("  pwd        Print working directory")
			fmt.Println("  cat <_id>  Print a document")
			fmt.Println("  rm <_id>   Delete a document")
			fmt.Println("  exit       Exit mongobash")
		default:
			fmt.Printf("Unknown command: %s\n", command)
		}
	}
}
