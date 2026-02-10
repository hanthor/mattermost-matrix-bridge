# Security Fixes Implementation Guide

This document provides detailed implementation guidance for addressing the security issues identified in SECURITY_REVIEW.md.

## Table of Contents

1. [Critical Fixes](#critical-fixes)
2. [High Priority Fixes](#high-priority-fixes)
3. [Testing Security Fixes](#testing-security-fixes)
4. [Deployment Checklist](#deployment-checklist)

---

## Critical Fixes

### Fix #1: Secure Password Generation

**File**: `mattermost/matrix_admin.go`  
**Lines**: 275-282

**Current (Vulnerable) Code**:
```go
func randomString(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[i%len(charset)] // Deterministic!
    }
    return string(b)
}
```

**Secure Implementation**:
```go
import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "math/big"
)

// GenerateSecurePassword generates a cryptographically secure random password
func GenerateSecurePassword() string {
    // Option 1: Base64-encoded random bytes (cleaner, URL-safe)
    const passwordLength = 32 // 256 bits of entropy
    bytes := make([]byte, passwordLength)
    if _, err := rand.Read(bytes); err != nil {
        panic(fmt.Sprintf("Failed to generate secure password: %v", err))
    }
    // Use URL-safe base64 encoding and remove padding
    return base64.RawURLEncoding.EncodeToString(bytes)
}

// GenerateReadablePassword generates a pronounceable password using crypto/rand
func GenerateReadablePassword(length int) (string, error) {
    // Option 2: Random selection from charset (more readable)
    const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*-_=+"
    if length < 16 {
        return "", fmt.Errorf("password length must be at least 16 characters")
    }
    
    b := make([]byte, length)
    charsetLen := big.NewInt(int64(len(charset)))
    
    for i := range b {
        num, err := rand.Int(rand.Reader, charsetLen)
        if err != nil {
            return "", fmt.Errorf("failed to generate random number: %w", err)
        }
        b[i] = charset[num.Int64()]
    }
    
    return string(b), nil
}

// Deprecated: Use GenerateSecurePassword instead
func randomString(length int) string {
    panic("randomString is deprecated and insecure. Use GenerateSecurePassword instead.")
}
```

**Update GeneratePassword function** (line 268):
```go
func GeneratePassword() string {
    // Use the new secure generation
    password := GenerateSecurePassword()
    return password
}
```

**Testing**:
```go
func TestGenerateSecurePassword(t *testing.T) {
    // Test uniqueness
    passwords := make(map[string]bool)
    for i := 0; i < 1000; i++ {
        pwd := GenerateSecurePassword()
        if passwords[pwd] {
            t.Fatal("Generated duplicate password")
        }
        passwords[pwd] = true
        
        // Verify minimum length (base64 of 32 bytes = ~43 chars)
        if len(pwd) < 32 {
            t.Fatalf("Password too short: %d", len(pwd))
        }
    }
}

func TestGenerateReadablePassword(t *testing.T) {
    pwd, err := GenerateReadablePassword(20)
    if err != nil {
        t.Fatal(err)
    }
    if len(pwd) != 20 {
        t.Fatalf("Expected length 20, got %d", len(pwd))
    }
    
    // Test that it rejects short lengths
    _, err = GenerateReadablePassword(10)
    if err == nil {
        t.Fatal("Should reject password length < 16")
    }
}
```

---

### Fix #2: Enforce Token Authentication

**File**: `mattermost/slashcmd.go`  
**Lines**: 82-85

**Current (Vulnerable) Code**:
```go
// Verify token if configured
if h.Token != "" && req.Token != h.Token {
    http.Error(w, "Unauthorized", http.StatusUnauthorized)
    return
}
```

**Secure Implementation**:
```go
import (
    "crypto/subtle"
)

// verifyToken performs constant-time token comparison
func (h *SlashCommandHandler) verifyToken(providedToken string) error {
    // CRITICAL: Token must be configured
    if h.Token == "" {
        return fmt.Errorf("server misconfiguration: authentication token not configured")
    }
    
    // Use constant-time comparison to prevent timing attacks
    if subtle.ConstantTimeCompare([]byte(h.Token), []byte(providedToken)) != 1 {
        return fmt.Errorf("invalid authentication token")
    }
    
    return nil
}

// Updated ServeHTTP
func (h *SlashCommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // ... existing code ...
    
    // Verify token (MANDATORY)
    if err := h.verifyToken(req.Token); err != nil {
        // Log the attempt without exposing details
        fmt.Printf("SECURITY: Unauthorized slash command attempt from %s: %v\n", 
            r.RemoteAddr, err)
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // ... rest of handler ...
}
```

**Startup Validation** in `mattermost/connector.go:260`:
```go
func (m *MattermostConnector) startSlashCommandServer() {
    // CRITICAL: Validate token before starting server
    if m.Config.SlashCommandToken == "" {
        fmt.Printf("FATAL: slash_command_token not configured. Server will not start.\n")
        fmt.Printf("To fix: Add a random token to config.yaml under network.slash_command_token\n")
        fmt.Printf("Example: slash_command_token: \"%s\"\n", GenerateSecurePassword())
        panic("slash_command_token is required for security")
    }
    
    if len(m.Config.SlashCommandToken) < 32 {
        fmt.Printf("WARNING: slash_command_token is too short (%d chars). Use at least 32 characters.\n", 
            len(m.Config.SlashCommandToken))
        fmt.Printf("Generate a new token: %s\n", GenerateSecurePassword())
    }
    
    handler := NewSlashCommandHandler(m, m.Config.SlashCommandToken)
    // ... rest of function ...
}
```

**Configuration Validation**:
Add to `example-config.yaml`:
```yaml
network:
    server_url: ""
    admin_token: ""
    # REQUIRED: Random token for slash command authentication
    # Generate with: openssl rand -base64 32
    # NEVER commit this token to version control!
    slash_command_token: "CHANGE_ME_TO_RANDOM_STRING"
```

**Testing**:
```go
func TestSlashCommandTokenValidation(t *testing.T) {
    connector := &MattermostConnector{
        Config: &NetworkConfig{
            SlashCommandToken: "",
        },
    }
    
    // Should panic if token not configured
    defer func() {
        if r := recover(); r == nil {
            t.Fatal("Expected panic when token not configured")
        }
    }()
    
    connector.startSlashCommandServer()
}

func TestTokenVerification(t *testing.T) {
    handler := &SlashCommandHandler{
        Token: "test-secure-token-12345678",
    }
    
    // Valid token
    err := handler.verifyToken("test-secure-token-12345678")
    if err != nil {
        t.Fatalf("Valid token rejected: %v", err)
    }
    
    // Invalid token
    err = handler.verifyToken("wrong-token")
    if err == nil {
        t.Fatal("Invalid token accepted")
    }
    
    // Empty token
    err = handler.verifyToken("")
    if err == nil {
        t.Fatal("Empty token accepted")
    }
}
```

---

### Fix #3: Encrypt Stored Credentials

**New File**: `mattermost/encryption.go`

```go
package mattermost

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
    "os"
)

// CredentialEncryption handles encryption/decryption of sensitive data
type CredentialEncryption struct {
    key []byte
    gcm cipher.AEAD
}

// NewCredentialEncryption creates a new encryption handler
func NewCredentialEncryption() (*CredentialEncryption, error) {
    // Get encryption key from environment variable
    keyString := os.Getenv("BRIDGE_ENCRYPTION_KEY")
    if keyString == "" {
        return nil, fmt.Errorf("BRIDGE_ENCRYPTION_KEY environment variable not set")
    }
    
    // Decode base64 key
    key, err := base64.StdEncoding.DecodeString(keyString)
    if err != nil {
        return nil, fmt.Errorf("invalid encryption key format: %w", err)
    }
    
    // Validate key length (must be 16, 24, or 32 bytes for AES)
    if len(key) != 32 {
        return nil, fmt.Errorf("encryption key must be 32 bytes (256 bits), got %d", len(key))
    }
    
    // Create AES cipher
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("failed to create cipher: %w", err)
    }
    
    // Create GCM mode (authenticated encryption)
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, fmt.Errorf("failed to create GCM: %w", err)
    }
    
    return &CredentialEncryption{
        key: key,
        gcm: gcm,
    }, nil
}

// GenerateKey generates a new encryption key
func GenerateKey() (string, error) {
    key := make([]byte, 32) // 256 bits
    if _, err := rand.Read(key); err != nil {
        return "", err
    }
    return base64.StdEncoding.EncodeToString(key), nil
}

// Encrypt encrypts plaintext using AES-GCM
func (ce *CredentialEncryption) Encrypt(plaintext string) (string, error) {
    if plaintext == "" {
        return "", nil
    }
    
    // Generate nonce
    nonce := make([]byte, ce.gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    // Encrypt
    ciphertext := ce.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    
    // Encode to base64 for storage
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-GCM
func (ce *CredentialEncryption) Decrypt(ciphertext string) (string, error) {
    if ciphertext == "" {
        return "", nil
    }
    
    // Decode from base64
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", fmt.Errorf("failed to decode ciphertext: %w", err)
    }
    
    // Extract nonce
    nonceSize := ce.gcm.NonceSize()
    if len(data) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }
    
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    
    // Decrypt
    plaintext, err := ce.gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", fmt.Errorf("decryption failed: %w", err)
    }
    
    return string(plaintext), nil
}
```

**Update login.go**:
```go
func (p *PATLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
    token := input["token"]
    
    // Encrypt token before storing
    encryption, err := NewCredentialEncryption()
    if err != nil {
        return nil, fmt.Errorf("encryption not available: %w", err)
    }
    
    encryptedToken, err := encryption.Encrypt(token)
    if err != nil {
        return nil, fmt.Errorf("failed to encrypt token: %w", err)
    }
    
    // ... existing user fetch code ...
    
    return &bridgev2.LoginStep{
        Type: bridgev2.LoginStepTypeComplete,
        CompleteParams: &bridgev2.LoginCompleteParams{
            UserLoginID: networkid.UserLoginID(me.Username),
            UserLogin: &bridgev2.UserLogin{
                UserLogin: &database.UserLogin{
                    Metadata: map[string]any{
                        "token": encryptedToken,  // Store encrypted
                        "mm_id": me.Id,
                    },
                    RemoteName: me.Username,
                },
            },
        },
    }, nil
}
```

**Update connector.go** to decrypt tokens:
```go
func (m *MattermostConnector) NewNetworkAPI(login *bridgev2.UserLogin) (bridgev2.NetworkAPI, error) {
    // ... existing code ...
    
    if login != nil {
        meta, ok := login.Metadata.(map[string]any)
        if ok {
            if encryptedToken, ok := meta["token"].(string); ok && encryptedToken != "" {
                // Decrypt token
                encryption, err := NewCredentialEncryption()
                if err != nil {
                    return nil, fmt.Errorf("encryption not available: %w", err)
                }
                
                token, err := encryption.Decrypt(encryptedToken)
                if err != nil {
                    return nil, fmt.Errorf("failed to decrypt token: %w", err)
                }
                
                api.Client = NewClient(m.Config.ServerURL, token)
            }
        }
    }
    
    // ... rest of function ...
}
```

**Documentation** for deployment:
```bash
# Generate encryption key
echo "BRIDGE_ENCRYPTION_KEY=$(openssl rand -base64 32)" >> .env

# Or in Go:
# key, _ := mattermost.GenerateKey()
# export BRIDGE_ENCRYPTION_KEY=$key
```

**Testing**:
```go
func TestEncryption(t *testing.T) {
    // Set test key
    os.Setenv("BRIDGE_ENCRYPTION_KEY", "VGhpcyBpcyBhIHRlc3Qga2V5IGZvciBBRVMtMjU2IQ==")
    defer os.Unsetenv("BRIDGE_ENCRYPTION_KEY")
    
    encryption, err := NewCredentialEncryption()
    if err != nil {
        t.Fatal(err)
    }
    
    plaintext := "my-secret-token-12345"
    
    // Encrypt
    ciphertext, err := encryption.Encrypt(plaintext)
    if err != nil {
        t.Fatal(err)
    }
    
    // Should be different
    if ciphertext == plaintext {
        t.Fatal("Ciphertext equals plaintext")
    }
    
    // Decrypt
    decrypted, err := encryption.Decrypt(ciphertext)
    if err != nil {
        t.Fatal(err)
    }
    
    // Should match
    if decrypted != plaintext {
        t.Fatalf("Decryption failed: got %s, want %s", decrypted, plaintext)
    }
}
```

---

## High Priority Fixes

### Fix #4: Rate Limiting

**New File**: `mattermost/ratelimit.go`

```go
package mattermost

import (
    "net/http"
    "sync"
    "time"
    
    "golang.org/x/time/rate"
)

// RateLimiter implements per-IP rate limiting
type RateLimiter struct {
    limiters map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
    
    // Cleanup old entries
    lastCleanup time.Time
}

// NewRateLimiter creates a new rate limiter
// rate: requests per second per IP
// burst: maximum burst size
func NewRateLimiter(rps float64, burst int) *RateLimiter {
    return &RateLimiter{
        limiters:    make(map[string]*rate.Limiter),
        rate:        rate.Limit(rps),
        burst:       burst,
        lastCleanup: time.Now(),
    }
}

// getLimiter returns the rate limiter for the given IP
func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    // Periodic cleanup
    if time.Since(rl.lastCleanup) > 5*time.Minute {
        rl.cleanup()
    }
    
    limiter, exists := rl.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.limiters[ip] = limiter
    }
    
    return limiter
}

// cleanup removes limiters that haven't been used recently
func (rl *RateLimiter) cleanup() {
    // Remove all limiters (they'll be recreated if needed)
    // In production, track last access time for each limiter
    rl.limiters = make(map[string]*rate.Limiter)
    rl.lastCleanup = time.Now()
}

// Allow checks if a request from the given IP should be allowed
func (rl *RateLimiter) Allow(ip string) bool {
    limiter := rl.getLimiter(ip)
    return limiter.Allow()
}

// Middleware wraps an HTTP handler with rate limiting
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get client IP (handle X-Forwarded-For for proxies)
        ip := r.RemoteAddr
        if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
            ip = xff
        }
        // Strip port
        if colonIdx := len(ip) - 1; colonIdx >= 0 {
            for i := len(ip) - 1; i >= 0; i-- {
                if ip[i] == ':' {
                    ip = ip[:i]
                    break
                }
            }
        }
        
        if !rl.Allow(ip) {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

**Update connector.go**:
```go
func (m *MattermostConnector) startSlashCommandServer() {
    // ... validation code ...
    
    handler := NewSlashCommandHandler(m, m.Config.SlashCommandToken)
    
    // Create rate limiter: 10 requests per second per IP, burst of 20
    rateLimiter := NewRateLimiter(10, 20)
    
    mux := http.NewServeMux()
    mux.Handle("/mattermost/command", handler)
    
    // Wrap with rate limiting
    rateLimitedHandler := rateLimiter.Middleware(mux)
    
    addr := ":8081"
    fmt.Printf("INFO: Starting slash command server on %s with rate limiting (10 req/s)\n", addr)
    
    server := &http.Server{
        Addr:         addr,
        Handler:      rateLimitedHandler,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 10 * time.Second,
        IdleTimeout:  60 * time.Second,
    }
    
    if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        fmt.Printf("ERROR: Slash command server failed: %v\n", err)
    }
}
```

---

### Fix #5: Input Validation

**New File**: `mattermost/validation.go`

```go
package mattermost

import (
    "fmt"
    "regexp"
    "strings"
)

var (
    // Matrix ID pattern: @localpart:domain
    matrixUserIDPattern = regexp.MustCompile(`^@[a-z0-9._=\-/]+:[a-z0-9.-]+\.[a-z]{2,}$`)
    
    // Room alias pattern: #localpart:domain
    roomAliasPattern = regexp.MustCompile(`^#[a-z0-9._=\-/]+:[a-z0-9.-]+\.[a-z]{2,}$`)
    
    // Room ID pattern: !opaque_id:domain
    roomIDPattern = regexp.MustCompile(`^![a-zA-Z0-9]+:[a-z0-9.-]+\.[a-z]{2,}$`)
    
    // Safe domain pattern (no underscores, special chars)
    domainPattern = regexp.MustCompile(`^[a-z0-9.-]+\.[a-z]{2,}$`)
)

// ValidationError represents a validation failure
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateMatrixUserID validates a Matrix user ID
func ValidateMatrixUserID(userID string) error {
    if len(userID) == 0 {
        return &ValidationError{"user_id", "cannot be empty"}
    }
    if len(userID) > 255 {
        return &ValidationError{"user_id", "exceeds maximum length of 255 characters"}
    }
    if !strings.HasPrefix(userID, "@") {
        return &ValidationError{"user_id", "must start with @"}
    }
    if !strings.Contains(userID, ":") {
        return &ValidationError{"user_id", "must contain : separator"}
    }
    if !matrixUserIDPattern.MatchString(strings.ToLower(userID)) {
        return &ValidationError{"user_id", "invalid format (expected: @localpart:domain.com)"}
    }
    
    // Validate domain part
    parts := strings.SplitN(userID, ":", 2)
    if len(parts) != 2 {
        return &ValidationError{"user_id", "missing domain"}
    }
    
    domain := parts[1]
    if !domainPattern.MatchString(domain) {
        return &ValidationError{"user_id", "invalid domain format"}
    }
    
    // Prevent common attack vectors
    if strings.Contains(userID, "..") {
        return &ValidationError{"user_id", "contains invalid path traversal sequence"}
    }
    
    return nil
}

// ValidateRoomIdentifier validates a Matrix room alias or ID
func ValidateRoomIdentifier(identifier string) error {
    if len(identifier) == 0 {
        return &ValidationError{"room", "cannot be empty"}
    }
    if len(identifier) > 255 {
        return &ValidationError{"room", "exceeds maximum length of 255 characters"}
    }
    
    if strings.HasPrefix(identifier, "#") {
        if !roomAliasPattern.MatchString(strings.ToLower(identifier)) {
            return &ValidationError{"room", "invalid room alias format (expected: #room:domain.com)"}
        }
    } else if strings.HasPrefix(identifier, "!") {
        if !roomIDPattern.MatchString(identifier) {
            return &ValidationError{"room", "invalid room ID format (expected: !id:domain.com)"}
        }
    } else {
        return &ValidationError{"room", "must start with # (alias) or ! (room ID)"}
    }
    
    // Prevent common attack vectors
    if strings.Contains(identifier, "..") {
        return &ValidationError{"room", "contains invalid path traversal sequence"}
    }
    
    return nil
}

// SanitizeChannelName sanitizes a string for use as a Mattermost channel name
func SanitizeChannelName(name string) string {
    // Mattermost channel names: lowercase alphanumeric, hyphens, underscores
    // Max 64 characters
    name = strings.ToLower(name)
    name = regexp.MustCompile(`[^a-z0-9\-_]`).ReplaceAllString(name, "_")
    
    // Truncate if too long
    if len(name) > 64 {
        name = name[:64]
    }
    
    // Ensure it doesn't start/end with special chars
    name = strings.Trim(name, "_-")
    
    // Ensure minimum length
    if len(name) < 1 {
        name = "channel"
    }
    
    return name
}
```

**Update slashcmd.go**:
```go
func (h *SlashCommandHandler) dmResponse(ctx context.Context, userID, teamDomain string, args []string) *SlashCommandResponse {
    if len(args) == 0 {
        return &SlashCommandResponse{
            ResponseType: "ephemeral",
            Text:         "Usage: `/matrix dm <user>` - e.g., `/matrix dm @alice:matrix.org`",
        }
    }
    
    matrixUserID := args[0]
    
    // VALIDATE INPUT
    if err := ValidateMatrixUserID(matrixUserID); err != nil {
        return &SlashCommandResponse{
            ResponseType: "ephemeral",
            Text:         fmt.Sprintf("❌ Invalid Matrix user ID: %v", err),
        }
    }
    
    // ... rest of function ...
}

func (h *SlashCommandHandler) joinResponse(ctx context.Context, userID string, args []string) *SlashCommandResponse {
    if len(args) == 0 {
        return &SlashCommandResponse{
            ResponseType: "ephemeral",
            Text:         "Usage: `/matrix join <room>` - e.g., `/matrix join #test:matrix.org`",
        }
    }
    
    roomIdentifier := args[0]
    
    // VALIDATE INPUT
    if err := ValidateRoomIdentifier(roomIdentifier); err != nil {
        return &SlashCommandResponse{
            ResponseType: "ephemeral",
            Text:         fmt.Sprintf("❌ Invalid room identifier: %v", err),
        }
    }
    
    // ... rest of function ...
}
```

**Testing**:
```go
func TestValidateMatrixUserID(t *testing.T) {
    validIDs := []string{
        "@alice:matrix.org",
        "@bob:example.com",
        "@user_name:server.example.org",
    }
    
    for _, id := range validIDs {
        if err := ValidateMatrixUserID(id); err != nil {
            t.Errorf("Valid ID rejected: %s - %v", id, err)
        }
    }
    
    invalidIDs := []string{
        "alice:matrix.org",           // Missing @
        "@alice",                      // Missing domain
        "@alice:matrix",               // Invalid domain
        "@alice@matrix.org",           // @ instead of :
        "@alice:matrix..org",          // Path traversal
        "@alice:MATRIX.ORG",           // Uppercase (should be handled)
        strings.Repeat("a", 300),      // Too long
        "@alice:192.168.1.1",          // IP address (could be allowed if needed)
    }
    
    for _, id := range invalidIDs {
        if err := ValidateMatrixUserID(id); err == nil {
            t.Errorf("Invalid ID accepted: %s", id)
        }
    }
}
```

---

## Testing Security Fixes

### Unit Tests

Create `mattermost/security_test.go`:
```go
package mattermost

import (
    "testing"
)

func TestSecurityFixes(t *testing.T) {
    t.Run("PasswordGeneration", TestGenerateSecurePassword)
    t.Run("TokenValidation", TestTokenVerification)
    t.Run("Encryption", TestEncryption)
    t.Run("RateLimiting", TestRateLimiter)
    t.Run("InputValidation", TestValidateMatrixUserID)
}

func TestRateLimiter(t *testing.T) {
    rl := NewRateLimiter(10, 2) // 10 req/s, burst 2
    
    ip := "192.168.1.1"
    
    // First 2 should succeed (burst)
    if !rl.Allow(ip) {
        t.Fatal("First request denied")
    }
    if !rl.Allow(ip) {
        t.Fatal("Second request denied")
    }
    
    // Third should fail (rate limit)
    if rl.Allow(ip) {
        t.Fatal("Third request should be rate limited")
    }
}
```

### Integration Tests

Test the complete authentication flow:
```bash
# Test with valid token
curl -X POST http://localhost:8081/mattermost/command \
  -d "token=YOUR_TOKEN" \
  -d "command=/matrix" \
  -d "text=help"

# Test with invalid token (should fail)
curl -X POST http://localhost:8081/mattermost/command \
  -d "token=WRONG" \
  -d "command=/matrix" \
  -d "text=help"

# Test rate limiting
for i in {1..100}; do
  curl -X POST http://localhost:8081/mattermost/command \
    -d "token=YOUR_TOKEN" \
    -d "command=/matrix" \
    -d "text=help" &
done
wait
# Should see some 429 Too Many Requests responses
```

---

## Deployment Checklist

Before deploying the security fixes:

- [ ] Generate and securely store encryption key
  ```bash
  openssl rand -base64 32 > encryption_key.txt
  export BRIDGE_ENCRYPTION_KEY=$(cat encryption_key.txt)
  ```

- [ ] Generate and configure slash command token
  ```bash
  openssl rand -base64 32
  # Add to config.yaml: slash_command_token: <output>
  ```

- [ ] Test password generation
  ```bash
  go test -run TestGenerateSecurePassword
  ```

- [ ] Test token authentication
  ```bash
  go test -run TestTokenVerification
  ```

- [ ] Test encryption/decryption
  ```bash
  export BRIDGE_ENCRYPTION_KEY=$(openssl rand -base64 32)
  go test -run TestEncryption
  ```

- [ ] Test rate limiting
  ```bash
  go test -run TestRateLimiter
  ```

- [ ] Review all error messages for information disclosure

- [ ] Update documentation with security requirements

- [ ] Set up monitoring for:
  - Failed authentication attempts
  - Rate limit violations
  - Encryption errors

- [ ] Plan credential rotation schedule

---

## Migration Guide for Existing Deployments

If you have an existing deployment with unencrypted tokens:

1. **Generate encryption key**:
   ```bash
   export BRIDGE_ENCRYPTION_KEY=$(openssl rand -base64 32)
   # Save this key securely!
   ```

2. **Create migration script** (`scripts/migrate_tokens.go`):
   ```go
   // Script to encrypt existing tokens in database
   // Run once during upgrade
   ```

3. **Schedule downtime** for migration

4. **Backup database** before migration

5. **Run migration**, verify, then remove plaintext tokens

6. **Update all user tokens** (force re-login if needed)

---

## Next Steps

After implementing these critical fixes:

1. Address high-priority issues (rate limiting, input validation)
2. Implement audit logging
3. Add security headers
4. Set up dependency scanning
5. Schedule regular security reviews

For questions or assistance, please refer to the main SECURITY_REVIEW.md document.
