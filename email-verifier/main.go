package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"
)

const verifiedEmailsFile = "verified_emails.txt"

func main() {
	http.HandleFunc("/send-verification", corsMiddleware(sendVerificationHandler))
	http.HandleFunc("/verify", corsMiddleware(verifyEmailHandler))
	http.HandleFunc("/check-verification", corsMiddleware(checkVerificationHandler))

	fmt.Println("Service is running on port 8080...")
	http.ListenAndServe(":8080", nil)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:3000") // Allow your frontend origin
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func sendVerificationHandler(w http.ResponseWriter, r *http.Request) {
	/*
		curl -X POST http://localhost:8080/send-verification \
		-H "Content-Type: application/json" \
		-d '{"email": "example@example.com"}'

	*/
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	email := request.Email
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Check if the email is already verified
	if isEmailVerified(email) {
		http.Error(w, "Email is already verified", http.StatusConflict)
		return
	}

	// Send a verification email using SMTP
	verificationLink := fmt.Sprintf("http://localhost:8080/verify?email=%s", email)
	err := sendEmail(email, verificationLink)
	if err != nil {
		log.Println(err)
		http.Error(w, "Failed to send verification email", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"message": "Verification email sent successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	log.Printf("Successfully sent email verification to %v\n", email)
	json.NewEncoder(w).Encode(response)
}

func verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Check if the email is already verified
	if isEmailVerified(email) {
		http.Error(w, "Email is already verified", http.StatusConflict)
		return
	}

	// Append the email to the verified emails file
	if err := appendToFile(verifiedEmailsFile, email+"\n"); err != nil {
		http.Error(w, "Failed to verify email", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Email verified: %s\n", email)
	response := map[string]string{
		"message": "Email verified successfully",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func checkVerificationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	verified := isEmailVerified(email)
	response := map[string]interface{}{
		"email":    email,
		"verified": verified,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func isEmailVerified(email string) bool {
	file, err := os.Open(verifiedEmailsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		fmt.Printf("Error reading file: %v\n", err)
		return false
	}
	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Printf("Error reading file content: %v\n", err)
		return false
	}

	emails := strings.Split(string(content), "\n")
	for _, e := range emails {
		if e == email {
			return true
		}
	}
	return false
}

func appendToFile(filename, content string) error {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(content); err != nil {
		return err
	}
	return nil
}

func sendEmail(to string, verificationLink string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")

	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPassword == "" {
		return fmt.Errorf("SMTP environment variables are not set")
	}

	subject := "Svelte Commerce Email Verification"
	body := fmt.Sprintf(`<html><body><h1>Svelte Commerce Email Verification</h1><p>Click the link below to verify your email:</p><a href="%s">Verify Email</a></body></html>`, verificationLink)
	msg := fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nMIME-Version: 1.0\nContent-Type: text/html; charset=UTF-8\n\n%s", smtpUser, to, subject, body)

	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpHost)
	return smtp.SendMail(fmt.Sprintf("%s:%s", smtpHost, smtpPort), auth, smtpUser, []string{to}, []byte(msg))
}
