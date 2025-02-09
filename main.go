package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"go-web-server/internal/config"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ResponseWrapper struct {
	Data    interface{} `json:"data"`
	Status  string      `json:"status"`
	Message string      `json:"message"`
}

var (
	db     *sql.DB
	logger *log.Logger
)

func init() {
	// Initialize logger with timestamp
	logger = log.New(os.Stdout, "", 0)
}

func logRequest(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		startTime := time.Now().UTC()
		logger.Printf("[%s] %s %s - Started", 
			startTime.Format("2006-01-02 15:04:05"),
			r.Method, 
			r.URL.Path)
		
		handler(w, r)
		
		logger.Printf("[%s] %s %s - Completed", 
			time.Now().UTC().Format("2006-01-02 15:04:05"),
			r.Method, 
			r.URL.Path)
	}
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Database connection
	var err error
	db, err = sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		logger.Fatalf("[%s] Failed to connect to database: %v", 
			time.Now().UTC().Format("2006-01-02 15:04:05"), 
			err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Fatalf("[%s] Database connection failed: %v", 
			time.Now().UTC().Format("2006-01-02 15:04:05"), 
			err)
	}

	// Create users table if it doesn't exist
	createTable()

	// Initialize router
	r := mux.NewRouter()

	// Routes with logging middleware
	r.HandleFunc("/users", logRequest(createUser)).Methods("POST")
	r.HandleFunc("/users", logRequest(getUsers)).Methods("GET")
	r.HandleFunc("/users/{id}", logRequest(getUser)).Methods("GET")
	r.HandleFunc("/users/{id}", logRequest(updateUser)).Methods("PUT")
	r.HandleFunc("/users/{id}", logRequest(deleteUser)).Methods("DELETE")
	r.HandleFunc("/health", logRequest(healthCheck)).Methods("GET")

	// Start server
	serverAddr := fmt.Sprintf(":%s", cfg.ServerPort)
	logger.Printf("[%s] Server starting on port %s...", 
		time.Now().UTC().Format("2006-01-02 15:04:05"), 
		cfg.ServerPort)
	logger.Fatal(http.ListenAndServe(serverAddr, r))
}

func createTable() {
	// First, drop the existing table
	dropQuery := `DROP TABLE IF EXISTS users;`
	_, err := db.Exec(dropQuery)
	if err != nil {
	    logger.Fatalf("[%s] Failed to drop table: %v", 
		   time.Now().UTC().Format("2006-01-02 15:04:05"), 
		   err)
	}
 
	// Create the table with the new schema
	query := `
	    CREATE TABLE IF NOT EXISTS users (
		   id SERIAL PRIMARY KEY,
		   name VARCHAR(100) NOT NULL,
		   email VARCHAR(100) UNIQUE NOT NULL,
		   created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		   updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	    );`
 
	_, err = db.Exec(query)
	if err != nil {
	    logger.Fatalf("[%s] Failed to create table: %v", 
		   time.Now().UTC().Format("2006-01-02 15:04:05"), 
		   err)
	}
	
	logger.Printf("[%s] Database table 'users' created successfully", 
	    time.Now().UTC().Format("2006-01-02 15:04:05"))
 }

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response := ResponseWrapper{
		Data:    payload,
		Status:  fmt.Sprintf("%d", code),
		Message: http.StatusText(code),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	response := ResponseWrapper{
		Data:    nil,
		Status:  fmt.Sprintf("%d", code),
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(response)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	query := `
		INSERT INTO users (name, email, created_at, updated_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) 
		RETURNING id, name, email, created_at, updated_at`
	
	err := db.QueryRow(query, user.Name, user.Email).
		Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, user)
}

func getUsers(w http.ResponseWriter, r *http.Request) {
	var users []User

	query := `
		SELECT id, name, email, created_at, updated_at 
		FROM users 
		ORDER BY created_at DESC`
	
	rows, err := db.Query(query)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt); err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		users = append(users, user)
	}

	respondWithJSON(w, http.StatusOK, users)
}

func getUser(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var user User

	query := `
		SELECT id, name, email, created_at, updated_at 
		FROM users 
		WHERE id = $1`
	
	err := db.QueryRow(query, params["id"]).
		Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var user User
	
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	query := `
		UPDATE users 
		SET name = $1, email = $2, updated_at = CURRENT_TIMESTAMP 
		WHERE id = $3 
		RETURNING id, name, email, created_at, updated_at`
	
	err := db.QueryRow(query, user.Name, user.Email, params["id"]).
		Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	} else if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)

	query := `DELETE FROM users WHERE id = $1`
	result, err := db.Exec(query, params["id"])
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		respondWithError(w, http.StatusNotFound, "User not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().Format("2006-01-02 15:04:05"),
	})
}