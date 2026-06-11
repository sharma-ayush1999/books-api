package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Book struct {
	ID int `json:"id"`
	Title string `json:"title"`
	Author string `json:"author"`
}

type Store struct {
	mu sync.Mutex
	books []Book
	nextID int
}

func NewStore() *Store{
	return &Store{
		nextID: 1,
		books: []Book{},
	}
}

func (s *Store) listBooks(w http.ResponseWriter, r *http.Request){
	s.mu.Lock()
	defer s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.books)
}

func (s *Store) addBook(w http.ResponseWriter, r *http.Request){
	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	
	s.nextID++
	s.books = append(s.books, book)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)
}

func (s *Store) getBook(w http.ResponseWriter, r *http.Request){
	idStr := strings.TrimPrefix(r.URL.Path, "/books/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, b := range s.books {
		if b.ID == id {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(b)
			return
		}
	}

	http.Error(w, "book not found", http.StatusNotFound)
}

//test comment
func main(){
	store := NewStore()


	http.HandleFunc("/books/", func (w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			store.getBook(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	
	http.HandleFunc("/books", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			store.listBooks(w, r)
		case http.MethodPost:
			store.addBook(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}

	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request){
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"version":"2.0"}`)
	})

	port := os.Getenv("PORT")

	if port == "" {
		port = "9090"
	}
	log.Printf("Server starting on: %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}