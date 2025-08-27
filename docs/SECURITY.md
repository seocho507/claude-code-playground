# 🔒 Security Best Practices Guide

**Version**: 2.0  
**Last Updated**: 2025-08-27  
**Classification**: Internal Use Only

## 📋 Table of Contents
1. [Security Overview](#-security-overview)
2. [Authentication Security](#-authentication-security)
3. [Data Protection](#-data-protection)
4. [API Security](#-api-security)
5. [Infrastructure Security](#-infrastructure-security)
6. [Security Monitoring](#-security-monitoring)
7. [Incident Response](#-incident-response)
8. [Compliance Checklist](#-compliance-checklist)

---

## 🎯 Security Overview

### Security Principles
1. **Defense in Depth**: Multiple layers of security controls
2. **Least Privilege**: Minimal access rights for users and services
3. **Zero Trust**: Never trust, always verify
4. **Security by Design**: Security built into the architecture

### Current Security Features
- ✅ JWT-based authentication with refresh tokens
- ✅ Password hashing with bcrypt (cost factor 10)
- ✅ Rate limiting on sensitive endpoints
- ✅ Input validation and sanitization
- ✅ SQL injection protection via parameterized queries
- ✅ XSS protection headers
- ✅ CORS policy enforcement
- ✅ Session management with Redis
- ✅ TLS/SSL encryption in transit

---

## 🔐 Authentication Security

### JWT Token Security

#### Token Generation
```go
// Secure token configuration
const (
    AccessTokenExpiry  = 15 * time.Minute  // Short-lived
    RefreshTokenExpiry = 7 * 24 * time.Hour // Longer-lived
    TokenIssuer        = "auth-service"
)

// Use strong secrets (minimum 32 characters)
// Generate with: openssl rand -base64 32
```

#### Token Storage
```javascript
// Client-side best practices
// DO NOT store tokens in localStorage (XSS vulnerable)
// Use httpOnly cookies or memory storage

// Secure cookie example
document.cookie = `token=${token}; Secure; HttpOnly; SameSite=Strict`;
```

#### Token Validation
- Verify signature on every request
- Check expiration time
- Validate issuer and audience claims
- Implement token blacklisting for logout

### Password Security

#### Password Requirements
```go
// Enforced password policy
type PasswordPolicy struct {
    MinLength        int  // 8
    RequireUppercase bool // true
    RequireLowercase bool // true
    RequireNumbers   bool // true
    RequireSpecial   bool // false (optional)
    MaxLength        int  // 128
}
```

#### Password Hashing
```go
// Using bcrypt with appropriate cost factor
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    // Cost factor 10-12 for production
    hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
    return string(hash), err
}
```

#### Password Reset Security
- Use secure random tokens
- Expire tokens after 1 hour
- Single use tokens only
- Rate limit reset requests
- Send to verified email only

### Multi-Factor Authentication (MFA)

#### TOTP Implementation (Future)
```go
// Time-based One-Time Password
type TOTPConfig struct {
    Issuer    string
    Algorithm string // SHA1, SHA256, SHA512
    Digits    int    // 6 or 8
    Period    int    // 30 seconds
}
```

---

## 🛡️ Data Protection

### Encryption at Rest

#### Database Encryption
```sql
-- PostgreSQL Transparent Data Encryption (TDE)
-- Enable encryption for tablespaces
CREATE TABLESPACE encrypted_space 
  LOCATION '/encrypted/data' 
  WITH (encryption_key_id = 'key-id');
```

#### Sensitive Data Handling
```go
// Never log sensitive information
func LogRequest(req *Request) {
    log.Printf("User: %s, Action: %s", 
        req.UserID,  // OK
        req.Action)  // OK
    // Never log: passwords, tokens, PII
}
```

### Encryption in Transit

#### TLS Configuration
```yaml
# Minimum TLS 1.2
tls:
  minimum_version: "1.2"
  cipher_suites:
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
```

### Data Sanitization

#### Input Validation
```go
// Validate all user inputs
func ValidateEmail(email string) error {
    regex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
    if !regex.MatchString(strings.ToLower(email)) {
        return errors.New("invalid email format")
    }
    return nil
}

// Prevent SQL injection
query := "SELECT * FROM users WHERE email = $1"
db.QueryRow(query, email) // Parameterized query
```

#### Output Encoding
```go
// HTML escape for XSS prevention
import "html/template"

func RenderHTML(data string) string {
    return template.HTMLEscapeString(data)
}
```

---

## 🌐 API Security

### Rate Limiting

#### Configuration
```go
// Rate limit configuration per endpoint
var RateLimits = map[string]RateLimit{
    "/api/v1/auth/login":    {Rate: 5, Per: time.Minute},
    "/api/v1/auth/register": {Rate: 3, Per: time.Minute},
    "/api/v1/auth/forgot":   {Rate: 3, Per: time.Hour},
}
```

### CORS Policy

#### Strict CORS Configuration
```go
// CORS middleware configuration
cors := cors.Config{
    AllowOrigins:     []string{"https://yourdomain.com"},
    AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
    AllowHeaders:     []string{"Authorization", "Content-Type"},
    ExposeHeaders:    []string{"X-Request-ID"},
    AllowCredentials: true,
    MaxAge:          12 * time.Hour,
}
```

### API Security Headers

#### Required Headers
```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("Content-Security-Policy", "default-src 'self'")
        c.Next()
    }
}
```

### Request Validation

#### Size Limits
```go
// Limit request body size
router.MaxMultipartMemory = 8 << 20 // 8 MB
```

#### Content Type Validation
```go
func ValidateContentType(c *gin.Context) {
    if c.ContentType() != "application/json" {
        c.AbortWithStatusJSON(400, gin.H{"error": "Invalid content type"})
        return
    }
}
```

---

## 🏗️ Infrastructure Security

### Container Security

#### Dockerfile Best Practices
```dockerfile
# Use specific version tags
FROM golang:1.23-alpine AS builder

# Run as non-root user
RUN adduser -D -g '' appuser

# Use multi-stage builds
FROM alpine:3.19
COPY --from=builder /app/main /app/main
USER appuser

# No sensitive data in images
# Use secrets management
```

#### Docker Security Scanning
```bash
# Scan for vulnerabilities
docker scan auth-service:latest

# Use Trivy for comprehensive scanning
trivy image auth-service:latest
```

### Database Security

#### Connection Security
```go
// Use SSL for database connections
dsn := fmt.Sprintf(
    "host=%s port=%d user=%s password=%s dbname=%s sslmode=require",
    host, port, user, password, dbname,
)
```

#### Access Control
```sql
-- Create application user with limited privileges
CREATE USER app_user WITH ENCRYPTED PASSWORD 'strong_password';
GRANT CONNECT ON DATABASE auth_db TO app_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO app_user;
-- Do NOT grant: CREATE, DROP, ALTER, TRUNCATE
```

### Network Security

#### Firewall Rules
```bash
# iptables example
# Allow only necessary ports
iptables -A INPUT -p tcp --dport 22 -j ACCEPT   # SSH
iptables -A INPUT -p tcp --dport 443 -j ACCEPT  # HTTPS
iptables -A INPUT -p tcp --dport 8001 -s 10.0.0.0/8 -j ACCEPT # Internal only
iptables -A INPUT -j DROP # Drop all others
```

#### Network Segmentation
```yaml
# Docker network isolation
networks:
  frontend:
    driver: bridge
  backend:
    driver: bridge
    internal: true  # No external access
```

---

## 📊 Security Monitoring

### Logging Strategy

#### Security Events to Log
```go
// Log authentication events
log.Info("Authentication attempt", 
    "user", email,
    "ip", clientIP,
    "success", success,
    "timestamp", time.Now())

// Log authorization failures
log.Warn("Authorization denied",
    "user", userID,
    "resource", resource,
    "action", action)

// Log security violations
log.Error("Security violation detected",
    "type", violationType,
    "source", sourceIP,
    "details", details)
```

### Metrics to Monitor

#### Security Metrics
```prometheus
# Failed login attempts
auth_login_failures_total{reason="invalid_password"}

# Token validation failures
auth_token_validation_failures_total{reason="expired"}

# Rate limit violations
rate_limit_exceeded_total{endpoint="/api/v1/auth/login"}

# Database connection failures
database_connection_errors_total{database="auth_db"}
```

### Alerting Rules

#### Critical Security Alerts
```yaml
groups:
  - name: security_alerts
    rules:
      - alert: BruteForceAttempt
        expr: rate(auth_login_failures_total[5m]) > 10
        annotations:
          summary: "Possible brute force attack detected"
          
      - alert: UnauthorizedAccessAttempt
        expr: rate(auth_unauthorized_total[5m]) > 20
        annotations:
          summary: "High rate of unauthorized access attempts"
          
      - alert: SQLInjectionAttempt
        expr: security_sql_injection_detected > 0
        annotations:
          summary: "SQL injection attempt detected"
```

---

## 🚨 Incident Response

### Incident Response Plan

#### 1. Detection & Analysis
```bash
# Check recent authentication failures
grep "Authentication failed" /var/log/auth-service.log | tail -100

# Check for suspicious patterns
grep -E "(DROP|DELETE|UNION|SELECT.*FROM)" /var/log/auth-service.log

# Monitor active connections
netstat -tuln | grep ESTABLISHED
```

#### 2. Containment
```bash
# Block suspicious IP
iptables -A INPUT -s <suspicious-ip> -j DROP

# Disable compromised account
UPDATE users SET is_active = false WHERE email = 'compromised@example.com';

# Revoke all sessions for user
DELETE FROM sessions WHERE user_id = '<user-id>';
```

#### 3. Eradication & Recovery
```bash
# Rotate secrets
docker secret rm jwt_secret
echo "new_secret" | docker secret create jwt_secret -

# Force password reset
UPDATE users SET must_reset_password = true WHERE compromised = true;

# Audit logs
SELECT * FROM audit_logs WHERE timestamp > NOW() - INTERVAL '24 hours';
```

### Security Incident Checklist
- [ ] Identify scope of incident
- [ ] Document timeline of events
- [ ] Preserve evidence (logs, dumps)
- [ ] Notify security team
- [ ] Implement containment measures
- [ ] Analyze root cause
- [ ] Apply fixes and patches
- [ ] Update security measures
- [ ] Document lessons learned

---

## ✅ Compliance Checklist

### GDPR Compliance
- [ ] Data minimization implemented
- [ ] Right to be forgotten (account deletion)
- [ ] Data portability (export user data)
- [ ] Privacy by design
- [ ] Consent management
- [ ] Data breach notification process

### OWASP Top 10 Protection
- [x] A01: Broken Access Control - JWT & RBAC
- [x] A02: Cryptographic Failures - TLS & bcrypt
- [x] A03: Injection - Parameterized queries
- [x] A04: Insecure Design - Security architecture
- [x] A05: Security Misconfiguration - Hardened configs
- [x] A06: Vulnerable Components - Dependency scanning
- [x] A07: Authentication Failures - Strong auth
- [x] A08: Data Integrity - Input validation
- [x] A09: Security Logging - Comprehensive logs
- [x] A10: SSRF - Input validation & network isolation

### Security Audit Checklist
- [ ] Penetration testing completed
- [ ] Vulnerability scanning passed
- [ ] Code security review done
- [ ] Dependency audit completed
- [ ] Access controls reviewed
- [ ] Encryption verified
- [ ] Backup/recovery tested
- [ ] Incident response tested

---

## 🔧 Security Tools

### Recommended Security Tools

#### Static Analysis
```bash
# Go security checker
gosec ./...

# Dependency vulnerability check
go list -json -m all | nancy sleuth

# License compliance
go-licenses check ./...
```

#### Dynamic Analysis
```bash
# OWASP ZAP API scan
zap-cli quick-scan --self-contained \
  --start-options '-config api.disablekey=true' \
  http://localhost:8001

# SQLMap for SQL injection testing
sqlmap -u "http://localhost:8001/api/v1/auth/login" \
  --data='{"email":"test","password":"test"}' \
  --batch --risk=1 --level=1
```

#### Container Security
```bash
# Trivy vulnerability scanner
trivy image auth-service:latest

# Docker Bench Security
docker run -it --net host --pid host \
  --cap-add audit_control \
  docker/docker-bench-security
```

---

## 📞 Security Contacts

| Role | Contact | When to Contact |
|------|---------|-----------------|
| Security Team Lead | security@example.com | Security incidents |
| CISO | ciso@example.com | Policy violations |
| DPO (Data Protection) | dpo@example.com | Data breaches |
| SOC (24/7) | soc@example.com | Active attacks |

---

## 📚 Additional Resources

- [OWASP Application Security Verification Standard](https://owasp.org/www-project-application-security-verification-standard/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
- [CIS Security Benchmarks](https://www.cisecurity.org/cis-benchmarks/)
- [Go Security Best Practices](https://github.com/OWASP/Go-SCP)

---

**Remember**: Security is everyone's responsibility. Report suspicious activities immediately!