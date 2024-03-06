package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/dpapathanasiou/go-recaptcha"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/time/rate"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
	"verifiedPetition/orm"
)

//go:embed static
var staticFS embed.FS

//go:embed www/**
var wwwFS embed.FS

//go:embed data
var dataFS embed.FS

var encryptionKey = []byte("")
var salt = []byte("")
var hostname = ""
var isHTTPS = false
var urlBase = ""
var allowedEmailDomains []string
var rateLimiters = sync.Map{}
var sendgridAPIKey string

func init() {
	sendgridAPIKey = os.Getenv("PETITION_SENDGRID_API_KEY")
	allowedEmailDomains = make([]string, 0, 1000)
	hostname = os.Getenv("PETITION_HOSTNAME")
	encryptionKey = []byte(os.Getenv("PETITION_ENC_KEY"))
	salt = []byte(os.Getenv("PETITION_SALT"))
	if hostname == "" {
		hostname = "localhost"
	}
	isHTTPS = strings.ToLower(os.Getenv("PETITION_HTTPS")) == "true"
	if isHTTPS {
		urlBase = "https://" + hostname
	} else {
		urlBase = "http://" + hostname
	}
	allowedEmailDomainsFile, err := dataFS.Open("data/allowed_email_domains.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer allowedEmailDomainsFile.Close()
	buff, err := io.ReadAll(allowedEmailDomainsFile)
	if err != nil {
		log.Fatal(err)
	}
	allowedWithEmpties := strings.Split(string(buff), "\n")
	var allowedWithoutEmpties []string
	// remove any empty strings
	for _, domain := range allowedWithEmpties {
		if domain != "" {
			allowedWithoutEmpties = append(allowedWithoutEmpties, domain)
		}
	}
	allowedEmailDomains = allowedWithoutEmpties
	recaptcha.Init(os.Getenv("PETITION_RECAPTCHA_SECRET"))
}

func validateKey(key []byte) []byte {
	// pad the key up to 32 bytes
	if len(key) < 32 {
		key = append(key, make([]byte, 32-len(key))...)
	}
	// truncate the key to 32 bytes
	if len(key) > 32 {
		key = key[:32]
	}
	return key
}

func decryptString(key []byte, encryptedString string) (string, error) {
	key = validateKey(key)
	ciphertext, _ := base64.StdEncoding.DecodeString(encryptedString)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", err
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func encryptString(key []byte, text string) (string, error) {
	key = validateKey(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(text), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func sendNotarizationEmail(email, encryptedForm string) {
	plainTextContent := fmt.Sprintf("%v/notarize?q=%v", urlBase, url.QueryEscape(encryptedForm))
	if strings.HasPrefix(urlBase, "http://localhost") {
		log.Println("Would have sent email to", email)
		log.Println(plainTextContent)
		return
	}
	from := mail.NewEmail("Professionals Against Genocide", "mail@stopthegenocide.world")
	subject := "Professionals Against Genocide"
	to := mail.NewEmail("", email)
	htmlContent := ""
	message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(sendgridAPIKey)
	_, err := client.Send(message)
	if err != nil {
		log.Println(err)
	} else {
	}
}

func rateLimitedHandler(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	remoteIp := strings.Split(r.RemoteAddr, ":")[0]
	_limiter, _ := rateLimiters.LoadOrStore(remoteIp, rate.NewLimiter(rate.Every(time.Second), 2))
	limiter := _limiter.(*rate.Limiter)
	if !limiter.Allow() {
		w.Header().Set("Retry-After", fmt.Sprintf("%v", limiter.Limit()))
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}
	next(w, r)
}

func validateCaptchav3(r *http.Request) bool {
	recaptchaResponse, responseFound := r.Form["g-recaptcha-response"]
	if responseFound {
		result, err := recaptcha.Confirm(r.RemoteAddr, recaptchaResponse[0])
		if err != nil {
			log.Println("recaptcha server error", err)
		}
		return result
	}
	return false
}

func _submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error parsing form:", err)
		return
	}
	passesCaptchav3 := validateCaptchav3(r)
	if !passesCaptchav3 {
		w.Write([]byte("failed recaptcha"))
		log.Println("Failed recaptcha")
		return
	}
	r.Form.Del("g-recaptcha-response")
	// check the email domain
	email := r.Form.Get("email")
	allowed := verifyEmailDomain(email)
	if !allowed {
		http.Redirect(w, r, "/domain_not_accepted.html", http.StatusFound)
		return
	}
	// replace the email in the form if the user asked for it
	anonymize := r.Form.Get("anonymize")
	emailSplit := strings.Split(email, "@")
	emailNameWithoutPlusAlias := strings.Split(emailSplit[0], "+")[0]
	if len(emailNameWithoutPlusAlias) > 100 {
		http.Redirect(w, r, "/suspicious_email.html", http.StatusFound)
		return
	}
	if anonymize == "on" {
		emailHash := sha256.Sum256([]byte(emailNameWithoutPlusAlias))
		r.Form.Set(
			"email",
			fmt.Sprintf("anon-%v@%v", emailHash, emailSplit[1]),
		)
	} else {
		r.Form.Set("email", strings.Join([]string{emailNameWithoutPlusAlias, emailSplit[1]}, "@"))
	}
	r.Form.Set("expiry", time.Now().Add(time.Hour).Format(time.RFC3339))
	r.Form.Del("anonymize")
	formString := r.Form.Encode()

	encryptedForm, err := encryptString(encryptionKey, formString)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error encrypting form:", err)
		return
	}
	go sendNotarizationEmail(email, encryptedForm)
	http.Redirect(w, r, "/success.html", http.StatusFound)
	return
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	rateLimitedHandler(w, r, _submitHandler)
}

func verifyEmailDomain(email string) bool {
	emailDomain := strings.Split(email, "@")[1]
	for _, domain := range allowedEmailDomains {
		if domain == emailDomain {
			return true
		}
	}
	return false
}

func notarizeHandler(w http.ResponseWriter, r *http.Request) {
	queryparam := r.URL.Query().Get("q")
	decryptedForm, err := decryptString(encryptionKey, queryparam)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error decrypting form:", err)
		return
	}
	parsedForm, err := url.ParseQuery(decryptedForm)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error parsing form:", err)
		return
	}

	// check the expiry
	expiry, err := time.Parse(time.RFC3339, parsedForm.Get("expiry"))
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error parsing expiry:", err)
		return
	}
	if time.Now().After(expiry) {
		http.Redirect(w, r, "/link_expired.html", http.StatusFound)
		return
	}

	// check the email domain
	email := parsedForm.Get("email")
	allowed := verifyEmailDomain(email)
	if !allowed {
		// this really shouldn't happen. the key has been compromised
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error: email domain not allowed")
		return
	}

	err = orm.AddSignature(parsedForm.Get("email"))
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error adding signature:", err)
	}
	http.Redirect(w, r, "/success_notarized.html", http.StatusFound)
	return
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := orm.GetSignatureStats()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error getting stats:", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	statsJson, err := json.Marshal(stats)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error marshalling stats:", err)
		return
	}
	_, err = w.Write(statsJson)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("Error writing stats:", err)
		return
	}
}

func main() {
	webFS, _ := fs.Sub(wwwFS, "www")
	http.Handle("/", http.FileServer(http.FS(webFS)))
	http.Handle("/static/", http.FileServer(http.FS(staticFS)))
	http.Handle("/submit", http.HandlerFunc(submitHandler))
	http.Handle("/notarize", http.HandlerFunc(notarizeHandler))
	http.Handle("/stats", http.HandlerFunc(statsHandler))
	parsedURL, err := url.Parse(urlBase)
	if err != nil {
		log.Fatal(err)
	}
	port := parsedURL.Port()
	scheme := parsedURL.Scheme
	if port == "" && scheme == "http" {
		port = "80"
	}
	if port == "" && scheme == "https" {
		port = "443"
	}
	if isHTTPS {
		m := &autocert.Manager{
			Cache:      autocert.DirCache("certs"),                   // Folder to store the certificates
			Prompt:     autocert.AcceptTOS,                           // Accepts the Terms of Service during account registration
			HostPolicy: autocert.HostWhitelist(parsedURL.Hostname()), // Replace with your domain
		}

		s := &http.Server{
			Addr:      ":https",
			TLSConfig: m.TLSConfig(),
			Handler:   nil, // Your handler
		}

		log.Printf("Serving http/https for domains: %s\n", hostname)
		go http.ListenAndServe(":http", m.HTTPHandler(nil))
		log.Fatal(s.ListenAndServeTLS("", ""))
	} else {
		log.Printf("Serving http for domains: %s through port %s\n", hostname, port)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
	}
}
