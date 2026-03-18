package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (s *AppState) Do(line []rune, pos int) (newLine [][]rune, length int) {
	l := string(line[:pos])
	words := strings.Fields(l)
	isNewWord := len(l) > 0 && l[len(l)-1] == ' '

	if len(words) == 0 {
		return s.filterOptions([]string{"ls", "cd", "pwd", "cat", "rm", "clear", "exit", "help", "quit"}, ""), 0
	}

	cmd := words[0]
	lastWord := ""
	if !isNewWord {
		lastWord = words[len(words)-1]
	}

	if len(words) == 1 && !isNewWord {
		return s.filterOptions([]string{"ls", "cd", "pwd", "cat", "rm", "clear", "exit", "help", "quit"}, lastWord), len(lastWord)
	}

	if cmd == "cd" {
		opts := []string{"..", "/"}
		if s.currentDB == "" {
			dbs, _ := s.client.ListDatabaseNames(context.TODO(), bson.M{})
			opts = append(opts, dbs...)
		} else if s.currentCol == "" {
			db := s.client.Database(s.currentDB)
			cols, err := db.ListCollectionNames(context.TODO(), bson.M{})
			if err == nil {
				opts = append(opts, cols...)
			}
		}
		return s.filterOptions(opts, lastWord), len(lastWord)
	}

	if cmd == "cat" || cmd == "rm" {
		var opts []string
		if s.currentDB != "" && s.currentCol != "" {
			coll := s.client.Database(s.currentDB).Collection(s.currentCol)
			cursor, err := coll.Find(context.TODO(), bson.M{})
			if err == nil {
				defer cursor.Close(context.TODO())
				for cursor.Next(context.TODO()) {
					var doc bson.M
					if err := cursor.Decode(&doc); err == nil {
						idStr := fmt.Sprintf("%v", doc["_id"])
						if oid, ok := doc["_id"].(primitive.ObjectID); ok {
							idStr = oid.Hex()
						}
						opts = append(opts, idStr)
					}
				}
			}
		}
		return s.filterOptions(opts, lastWord), len(lastWord)
	}

	return nil, 0
}

func (s *AppState) filterOptions(options []string, prefix string) [][]rune {
	var res [][]rune
	for _, opt := range options {
		if strings.HasPrefix(opt, prefix) {
			// readline appends what we return to the current word.
			// so we must only return the remainder (suffix) of the matched option
			suffix := opt[len(prefix):]
			if suffix != "" {
				res = append(res, append([]rune(suffix), ' '))
			} else {
				res = append(res, []rune{' '})
			}
		}
	}
	return res
}

func (s *AppState) pwd() string {
	path := "/"
	if s.currentDB != "" {
		path += s.currentDB
		if s.currentCol != "" {
			path += "/" + s.currentCol
		}
	}
	return path
}

func (s *AppState) handleCd(target string) {
	if target == "/" || target == "~" {
		s.currentDB = ""
		s.currentCol = ""
		return
	}

	// Calculate absolute path representation
	currentPath := s.pwd()
	isAbsolute := strings.HasPrefix(target, "/") || strings.HasPrefix(target, "~/")

	var parts []string
	if isAbsolute {
		if strings.HasPrefix(target, "~/") {
			target = target[1:]
		}
		parts = []string{} // start at root
	} else {
		// convert current path to parts
		p := strings.Trim(currentPath, "/")
		if p != "" {
			parts = strings.Split(p, "/")
		}
	}

	targetParts := strings.Split(target, "/")
	for _, part := range targetParts {
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		} else {
			parts = append(parts, part)
		}
	}

	if len(parts) > 2 {
		fmt.Printf("mongobash: cd: %s: No such file or directory (max depth is db/collection)\n", target)
		return
	}

	// Apply new state
	if len(parts) == 0 {
		s.currentDB = ""
		s.currentCol = ""
	} else if len(parts) == 1 {
		s.currentDB = parts[0]
		s.currentCol = ""
	} else if len(parts) == 2 {
		s.currentDB = parts[0]
		s.currentCol = parts[1]
	}
}

func (s *AppState) handleLs() {
	if s.currentDB == "" {
		// List Databases
		dbs, err := s.client.ListDatabaseNames(context.TODO(), bson.M{})
		if err != nil {
			fmt.Printf("Error listing databases: %v\n", err)
			return
		}
		for _, db := range dbs {
			fmt.Printf("\033[34m%s\033[0m\n", db)
		}
	} else if s.currentCol == "" {
		// List Collections in currentDB
		db := s.client.Database(s.currentDB)
		cols, err := db.ListCollectionNames(context.TODO(), bson.M{})
		if err != nil {
			fmt.Printf("Error listing collections: %v\n", err)
			return
		}
		for _, col := range cols {
			fmt.Printf("\033[36m%s\033[0m\n", col)
		}
	} else {
		// List Documents in currentCol
		coll := s.client.Database(s.currentDB).Collection(s.currentCol)
		cursor, err := coll.Find(context.TODO(), bson.M{})
		if err != nil {
			fmt.Printf("Error listing documents: %v\n", err)
			return
		}
		defer cursor.Close(context.TODO())

		for cursor.Next(context.TODO()) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				fmt.Printf("Error decoding document: %v\n", err)
				continue
			}
			idStr := fmt.Sprintf("%v", doc["_id"])
			if oid, ok := doc["_id"].(primitive.ObjectID); ok {
				idStr = oid.Hex()
			}
			fmt.Printf("\033[32m%s\033[0m\n", idStr)
		}
	}
}

func (s *AppState) parseID(idStr string) interface{} {
	if oid, err := primitive.ObjectIDFromHex(idStr); err == nil {
		return oid
	}
	if val, err := strconv.Atoi(idStr); err == nil {
		return val
	}
	return idStr
}

func (s *AppState) handleCat(idStr string) {
	if s.currentCol == "" {
		fmt.Println("Error: Not in a collection. cd into a collection first.")
		return
	}

	coll := s.client.Database(s.currentDB).Collection(s.currentCol)
	id := s.parseID(idStr)

	var doc bson.M
	err := coll.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Printf("Document with _id %s not found\n", idStr)
		} else {
			fmt.Printf("Error finding document: %v\n", err)
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "\033[35m%s\033[0m\t\033[36m%s\033[0m\n", "KEY", "VALUE")
	for k, v := range doc {
		fmt.Fprintf(w, "\033[35m%s\033[0m\t\033[36m%v\033[0m\n", k, v)
	}
	w.Flush()
}

func (s *AppState) handleRm(idStr string) {
	if s.currentCol == "" {
		fmt.Println("Error: Not in a collection. cd into a collection first.")
		return
	}

	coll := s.client.Database(s.currentDB).Collection(s.currentCol)
	id := s.parseID(idStr)

	res, err := coll.DeleteOne(context.TODO(), bson.M{"_id": id})
	if err != nil {
		fmt.Printf("Error deleting document: %v\n", err)
		return
	}
	if res.DeletedCount == 0 {
		fmt.Printf("Document with _id %s not found\n", idStr)
	} else {
		fmt.Printf("Deleted 1 document (id: %s)\n", idStr)
	}
}
