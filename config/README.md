# Configuration Directory Structure

## 📁 Directory Organization

```
config/
└── traefik/          # API Gateway configuration
    └── dynamic/
        └── dynamic.toml  # Dynamic routing configuration for unified auth service
```

## 🎯 Configuration Principles

### 1. Environment-Based Configuration
- Use environment variables with `${VAR:default}` pattern
- Separate configs for dev/staging/production
- Never commit secrets to version control

### 2. Unified Service Architecture
- Single auth-service handles all authentication and user operations
- Consolidated configuration in `services/auth-service/config/`
- Clear configuration ownership and reduced complexity

### 3. Dynamic Configuration
- Traefik uses dynamic configuration for routing
- Service discovery through Docker labels
- Hot-reload capability where possible

## 🔧 Usage Examples

### Loading Environment Variables
```bash
# Development
export $(grep -v '^#' .env.dev | xargs)

# Production  
export $(grep -v '^#' .env.prod | xargs)
```

### Service Configuration
```toml
# services/auth-service/config.toml
[database]
host = "${AUTH_DB_HOST:localhost}"
port = "${AUTH_DB_PORT:5432}"
name = "${AUTH_DB_NAME:auth_db}"
user = "${AUTH_DB_USER:postgres}"
password = "${AUTH_DB_PASSWORD}"

[redis]
url = "${REDIS_URL:redis://localhost:6379}"
```

### Traefik Dynamic Routing
```toml
# traefik/dynamic/dynamic.toml
[http.routers.auth-service]
rule = "Host(`localhost`) && PathPrefix(`/api/v1/auth`)"
service = "auth-service"
middlewares = ["default-headers", "global-ratelimit", "auth-ratelimit", "cors"]

[http.services.auth-service.loadBalancer]
[[http.services.auth-service.loadBalancer.servers]]
url = "http://auth-service:8080"
```