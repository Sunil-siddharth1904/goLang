package main

/*import (
	"fmt"
	"log"
	"net/http"
)

func main() {

	// API routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello world from GfG") // e.g. http://localhost:5000/
	})
	http.HandleFunc("/hi", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hi") // e.g. http://localhost:5000/hi
	})
	http.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name") // e.g. http://localhost:5000/greet?name=Sunil
		if name == "" {
			name = "Guest"
		}
		fmt.Fprintf(w, "Hello, %s!", name)
	})
	port := ":5000"
	fmt.Println("Server is running on port" + port)

	// Start server on port specified above
	log.Fatal(http.ListenAndServe(port, nil))
}
*/

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user in the system
type User struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Password  string    `json:"password,omitempty"` // omit in responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsActive  bool      `json:"is_active"`
}

// UserResponse represents user data for API responses (without password)
type UserResponse struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsActive  bool      `json:"is_active"`
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Password  string `json:"password"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Username  string `json:"username,omitempty"`
	Email     string `json:"email,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	IsActive  *bool  `json:"is_active,omitempty"`
}

// ChangePasswordRequest represents the request body for changing password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// In-memory storage (replace with database in production)
var users []User
var nextID = 1

// Email validation regex
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// Middleware for CORS
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Middleware for logging
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s - %v", r.Method, r.RequestURI, r.RemoteAddr, time.Since(start))
	})
}

// Middleware for JSON content type
func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// Helper function to convert User to UserResponse
func toUserResponse(user User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		IsActive:  user.IsActive,
	}
}

// Helper function to send JSON response
func sendResponse(w http.ResponseWriter, statusCode int, response APIResponse) {
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// Helper function to hash password
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Helper function to check password
func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Validation functions
func validateEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func validatePassword(password string) string {
	if len(password) < 8 {
		return "Password must be at least 8 characters long"
	}
	return ""
}

func validateCreateUserRequest(req CreateUserRequest) []string {
	var errors []string

	if req.Username == "" {
		errors = append(errors, "Username is required")
	} else if len(req.Username) < 3 {
		errors = append(errors, "Username must be at least 3 characters long")
	}

	if req.Email == "" {
		errors = append(errors, "Email is required")
	} else if !validateEmail(req.Email) {
		errors = append(errors, "Invalid email format")
	}

	if req.FirstName == "" {
		errors = append(errors, "First name is required")
	}

	if req.LastName == "" {
		errors = append(errors, "Last name is required")
	}

	if passwordError := validatePassword(req.Password); passwordError != "" {
		errors = append(errors, passwordError)
	}

	return errors
}

// Check if username or email already exists
func userExists(username, email string, excludeID int) bool {
	for _, user := range users {
		if user.ID != excludeID && (user.Username == username || user.Email == email) {
			return true
		}
	}
	return false
}

// Find user by ID
func findUserByID(id int) (*User, bool) {
	for i, user := range users {
		if user.ID == id {
			return &users[i], true
		}
	}
	return nil, false
}

// API Handlers

// GET /api/users - Get all users with pagination
func getUsers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	search := r.URL.Query().Get("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var filteredUsers []UserResponse
	for _, user := range users {
		if search == "" ||
			strings.Contains(strings.ToLower(user.Username), strings.ToLower(search)) ||
			strings.Contains(strings.ToLower(user.Email), strings.ToLower(search)) ||
			strings.Contains(strings.ToLower(user.FirstName+" "+user.LastName), strings.ToLower(search)) {
			filteredUsers = append(filteredUsers, toUserResponse(user))
		}
	}

	start := (page - 1) * limit
	end := start + limit

	if start > len(filteredUsers) {
		filteredUsers = []UserResponse{}
	} else if end > len(filteredUsers) {
		filteredUsers = filteredUsers[start:]
	} else {
		filteredUsers = filteredUsers[start:end]
	}

	response := APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"users": filteredUsers,
			"pagination": map[string]interface{}{
				"page":  page,
				"limit": limit,
				"total": len(users),
			},
		},
	}

	sendResponse(w, http.StatusOK, response)
}

// GET /api/users/{id} - Get user by ID
func getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	user, found := findUserByID(id)
	if !found {
		sendResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	sendResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    toUserResponse(*user),
	})
}

// POST /api/users - Create new user
func createUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON format",
		})
		return
	}

	// Validate input
	if errors := validateCreateUserRequest(req); len(errors) > 0 {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   strings.Join(errors, "; "),
		})
		return
	}

	// Check if user already exists
	if userExists(req.Username, req.Email, 0) {
		sendResponse(w, http.StatusConflict, APIResponse{
			Success: false,
			Error:   "Username or email already exists",
		})
		return
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		sendResponse(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Error processing password",
		})
		return
	}

	// Create user
	now := time.Now()
	user := User{
		ID:        nextID,
		Username:  req.Username,
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Password:  hashedPassword,
		CreatedAt: now,
		UpdatedAt: now,
		IsActive:  true,
	}

	users = append(users, user)
	nextID++

	sendResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Message: "User created successfully",
		Data:    toUserResponse(user),
	})
}

// PUT /api/users/{id} - Update user
func updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	user, found := findUserByID(id)
	if !found {
		sendResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON format",
		})
		return
	}

	// Update fields if provided
	if req.Username != "" {
		if userExists(req.Username, "", id) {
			sendResponse(w, http.StatusConflict, APIResponse{
				Success: false,
				Error:   "Username already exists",
			})
			return
		}
		user.Username = req.Username
	}

	if req.Email != "" {
		if !validateEmail(req.Email) {
			sendResponse(w, http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   "Invalid email format",
			})
			return
		}
		if userExists("", req.Email, id) {
			sendResponse(w, http.StatusConflict, APIResponse{
				Success: false,
				Error:   "Email already exists",
			})
			return
		}
		user.Email = req.Email
	}

	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}

	if req.LastName != "" {
		user.LastName = req.LastName
	}

	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	user.UpdatedAt = time.Now()

	sendResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "User updated successfully",
		Data:    toUserResponse(*user),
	})
}

// DELETE /api/users/{id} - Delete user (soft delete by setting IsActive to false)
func deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	user, found := findUserByID(id)
	if !found {
		sendResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	user.IsActive = false
	user.UpdatedAt = time.Now()

	sendResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "User deleted successfully",
	})
}

// PUT /api/users/{id}/password - Change user password
func changePassword(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid user ID",
		})
		return
	}

	user, found := findUserByID(id)
	if !found {
		sendResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "User not found",
		})
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid JSON format",
		})
		return
	}

	// Verify current password
	if !checkPassword(req.CurrentPassword, user.Password) {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Current password is incorrect",
		})
		return
	}

	// Validate new password
	if passwordError := validatePassword(req.NewPassword); passwordError != "" {
		sendResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   passwordError,
		})
		return
	}

	// Hash new password
	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		sendResponse(w, http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "Error processing password",
		})
		return
	}

	user.Password = hashedPassword
	user.UpdatedAt = time.Now()

	sendResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Password changed successfully",
	})
}

// GET /api/health - Health check
func healthCheck(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
			"version":   "1.0.0",
		},
	})
}

func main() {
	// Initialize with sample data
	hashedPassword, _ := hashPassword("password123")
	users = []User{
		{
			ID: 1, Username: "johndoe", Email: "john@example.com",
			FirstName: "John", LastName: "Doe", Password: hashedPassword,
			CreatedAt: time.Now(), UpdatedAt: time.Now(), IsActive: true,
		},
		{
			ID: 2, Username: "janesmith", Email: "jane@example.com",
			FirstName: "Jane", LastName: "Smith", Password: hashedPassword,
			CreatedAt: time.Now(), UpdatedAt: time.Now(), IsActive: true,
		},
	}
	nextID = 3

	// Create router
	r := mux.NewRouter()

	// Add middleware
	r.Use(corsMiddleware)
	r.Use(loggingMiddleware)
	r.Use(jsonMiddleware)

	// API routes
	api := r.PathPrefix("/api").Subrouter()

	api.HandleFunc("/health", healthCheck).Methods("GET")
	api.HandleFunc("/users", getUsers).Methods("GET")
	api.HandleFunc("/users/{id:[0-9]+}", getUser).Methods("GET")
	api.HandleFunc("/users", createUser).Methods("POST")
	api.HandleFunc("/users/{id:[0-9]+}", updateUser).Methods("PUT")
	api.HandleFunc("/users/{id:[0-9]+}", deleteUser).Methods("DELETE")
	api.HandleFunc("/users/{id:[0-9]+}/password", changePassword).Methods("PUT")

	// Handle preflight requests
	r.Methods("OPTIONS").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Start server
	port := ":8080"
	fmt.Printf("🚀 User API Server starting on port %s\n", port)
	fmt.Println("\n📋 Available endpoints:")
	fmt.Println("  GET    /api/health")
	fmt.Println("  GET    /api/users                    - Get all users (supports ?page=1&limit=10&search=term)")
	fmt.Println("  GET    /api/users/{id}              - Get user by ID")
	fmt.Println("  POST   /api/users                   - Create new user")
	fmt.Println("  PUT    /api/users/{id}              - Update user")
	fmt.Println("  DELETE /api/users/{id}              - Delete user (soft delete)")
	fmt.Println("  PUT    /api/users/{id}/password     - Change user password")
	fmt.Printf("\n🌐 Server running at http://localhost%s\n", port)

	log.Fatal(http.ListenAndServe(port, r))
}
