# 📁 Project Structure Guide

**Version**: 2.0  
**Last Updated**: 2025-08-27  
**Target Audience**: Developers, DevOps Engineers

## 🗂️ Overview

This document provides a comprehensive overview of the Auth Service project structure, explaining the purpose and organization of each directory and file.

## 📋 Root Directory Structure

```
backend/
├── 📄 README.md                    # Project overview and quick start
├── 📄 ONBOARDING.md                # 🚀 Complete developer onboarding guide
├── 📄 docker-compose.yml           # 🐳 Main Docker composition for development
├── 📄 .env.example                 # Environment variables template
├── 📄 .gitignore                   # Git ignore patterns
├── 📄 E2E_API_TESTING_PLAN.md     # 🧪 API testing documentation
├── 📄 Makefile                     # Build and development commands
├── 🔧 config/                      # Configuration files
├── 🐳 docker/                      # Docker-specific configurations
├── 📚 docs/                        # 📖 Documentation hub
├── 📜 scripts/                     # Utility scripts
├── 🛠️ services/                    # Microservices directory
├── 📦 shared/                      # Shared libraries and utilities
└── 🧪 test/                        # End-to-end tests
```

---

## 🛠️ Services Directory

### services/auth-service/
```
services/auth-service/
├── 📄 Dockerfile                   # Container image definition
├── 📄 README.md                    # Service-specific documentation
├── 📄 go.mod                       # Go module definition
├── 📄 go.sum                       # Go module checksums
├── 📄 main.go                      # 🚀 Application entry point
├── 🔧 cmd/                         # Command-line tools
│   └── migrate/                    # Database migration CLI
│       └── main.go                 # Migration tool entry point
├── 🔧 config/                      # Configuration files
│   ├── config.toml                 # Default configuration
│   ├── config-local.toml          # Local development config
│   ├── config-test.toml           # Test environment config
├── 🏗️ internal/                    # Internal packages (private)
│   ├── config/                     # Configuration management
│   │   └── config.go              # Config structure and loading
│   ├── database/                   # Database connection management
│   │   └── database.go            # Database initialization
│   ├── handlers/                   # 🌐 HTTP request handlers
│   │   ├── auth_handler.go        # Authentication endpoints
│   │   └── *_test.go              # Handler unit tests
│   ├── middleware/                 # 🔀 HTTP middleware
│   │   └── middleware.go          # CORS, logging, etc.
│   ├── models/                     # 📊 Data models
│   │   ├── user.go                # User model and validation
│   │   ├── requests.go            # Request/response structures
│   │   └── *_test.go              # Model unit tests
│   ├── repositories/               # 💾 Data access layer
│   │   ├── user_repository.go     # User data operations
│   │   ├── session_repository.go  # Session management
│   │   └── *_test.go              # Repository unit tests
│   └── services/                   # 🧠 Business logic layer
│       ├── auth_service.go        # Authentication business logic
│       ├── jwt_service.go         # JWT token management
│       ├── oauth2_service.go      # OAuth2 integration
│       └── *_test.go              # Service unit tests
├── 📊 migrations/                  # Database schema migrations
│   ├── 001_initial_schema.sql     # Initial database schema
│   └── 002_fix_sessions_table.sql # Schema updates
└── 🧪 tests/                       # Integration tests
    └── integration/                # Service-level integration tests
```

---

## 📦 Shared Directory

### shared/
```
shared/
├── 📄 go.mod                       # Shared module definition
├── 📄 go.sum                       # Shared module checksums
├── 🏗️ cache/                       # Caching utilities
│   └── cache_manager.go           # Redis cache management
├── 🔧 config/                      # Shared configuration
│   └── config.go                  # Common config structures
├── 💾 database/                    # Database utilities
│   ├── connection.go              # Database connection helper
│   └── redis.go                   # Redis connection helper
├── 📡 events/                      # Event bus system
│   └── event_bus.go               # Inter-service communication
├── 🏥 health/                      # Health check utilities
│   └── health.go                  # Health check endpoints
├── 🔀 middleware/                  # Shared HTTP middleware
│   ├── jwt_middleware.go          # 🔐 JWT authentication
│   ├── jwt_claims.go              # JWT claims structure
│   ├── middleware.go              # Common middleware
│   └── *_test.go                  # Middleware tests
├── 📨 redis/                       # Redis utilities
│   └── redis_manager.go           # Redis operation helpers
├── 🖥️ server/                      # Server utilities
│   └── server.go                  # HTTP server setup
└── 🔐 session/                     # Session management
    └── session_manager.go         # Session handling
```

---

## 📚 Documentation Directory

### docs/
```
docs/
├── 📄 README.md                    # Documentation index
├── 🚀 PRODUCTION_DEPLOYMENT.md     # 🚢 Production deployment guide
├── 🔒 SECURITY.md                  # 🛡️ Security best practices
├── 📁 PROJECT_STRUCTURE.md         # This file
├── 🗂️ completed-plans/             # Implementation history
│   ├── E2E_PRODUCTION_READINESS_PLAN.md
│   ├── MIGRATION_FIRST_PLAN.md
│   ├── PREVENTION_PLAN.md
│   ├── REFACTORING_PLAN.md
│   ├── SCHEMA_CONSISTENCY_PREVENTION_PLAN.md
│   └── SCHEMA_ISSUES_ANALYSIS.md
├── 📖 guides/                      # Developer guides
│   ├── MIGRATION_DEVELOPER_GUIDE.md
│   └── MIGRATION_QUICK_REFERENCE.md
└── 📚 reference/                   # Technical references
    ├── LESSONS_LEARNED.md
    ├── REDIS_BEST_PRACTICES.md
    ├── TEST_DOCUMENTATION.md
    └── TRAEFIK_FORWARDAUTH_ARCHITECTURE.md
```

---

## 🔧 Configuration Directory

### config/
```
config/
├── 📄 README.md                    # Configuration documentation
├── 📨 messaging/                   # Message queue configuration
│   └── redis.conf                 # Redis configuration
├── 📊 monitoring/                  # Monitoring setup
│   └── prometheus.yml             # Prometheus configuration
├── 🛠️ services/                    # Service-specific configs
│   └── auth-service/              # Auth service configuration
└── 🌐 traefik/                     # API Gateway configuration
    ├── dynamic.toml               # Dynamic configuration
    └── dynamic/                   # Additional configs
        └── dynamic.toml
```

---

## 🐳 Docker Directory

### docker/
```
docker/
├── docker-compose.legacy.yml       # Legacy configuration
├── docker-compose.migration-first.yml  # Migration-first setup
└── docker-compose.migration-test.yml   # Test environment
```

---

## 📜 Scripts Directory

### scripts/
```
scripts/
├── setup-git-hooks.sh             # 🔗 Git hooks installation
└── docker/                        # Docker-related scripts
    ├── migration-runner.sh        # Database migration runner
    └── post-migration-validation.sh # Migration validation
```

---

## 🧪 Test Directory

### test/
```
test/
├── 📄 go.mod                       # Test module definition
├── 📄 go.sum                       # Test module checksums
├── 🌐 traefik_forwardauth_test.go  # ForwardAuth integration tests
├── 🌐 traefik_routing_test.go      # API routing tests
└── 🐳 docker/                      # Test Docker setup
    ├── Dockerfile.auth-service     # Test container for auth service
    ├── Dockerfile.test-runner      # Test runner container
    ├── docker-compose.test.yml     # Test environment composition
    ├── final-api-test.sh          # Comprehensive API tests
    └── updated-api-test-results.txt # Test results
```

---

## 🎯 Key Design Principles

### Directory Organization
1. **Separation of Concerns**: Each directory has a specific responsibility
2. **Layered Architecture**: Clear separation between handlers, services, and repositories
3. **Shared Components**: Common utilities in shared directory
4. **Test Organization**: Tests close to the code they test

### File Naming Conventions
- `*_test.go`: Unit and integration tests
- `*_handler.go`: HTTP request handlers
- `*_service.go`: Business logic services
- `*_repository.go`: Data access layer
- `*.toml`: Configuration files
- `*.sql`: Database migration files

### Import Path Structure
```go
// Internal packages (private to auth-service)
import "auth-service/internal/handlers"
import "auth-service/internal/services"
import "auth-service/internal/models"

// Shared packages (reusable across services)
import "shared/middleware"
import "shared/database"
import "shared/cache"
```

---

## 🔍 Finding Your Way Around

### Common Tasks and Their Locations

#### Adding a New API Endpoint
1. **Handler**: `services/auth-service/internal/handlers/`
2. **Business Logic**: `services/auth-service/internal/services/`
3. **Data Model**: `services/auth-service/internal/models/`
4. **Database Operations**: `services/auth-service/internal/repositories/`
5. **Routes**: `services/auth-service/main.go`
6. **Tests**: Alongside the respective files (`*_test.go`)

#### Database Changes
1. **Migration Files**: `services/auth-service/migrations/`
2. **GORM Models**: `services/auth-service/internal/models/`
3. **Migration Tool**: `services/auth-service/cmd/migrate/`

#### Configuration Changes
1. **TOML Files**: `services/auth-service/config/`
2. **Go Structures**: `services/auth-service/internal/config/`
3. **Environment Variables**: `.env.example`

#### Deployment Configuration
1. **Docker Compose**: `docker-compose.yml` (development)
2. **Production Config**: `docker/docker-compose.prod.yml`
3. **Deployment Guide**: `docs/PRODUCTION_DEPLOYMENT.md`

### Documentation Hierarchy
```
📚 Documentation Priority Order:
1. 🚀 ONBOARDING.md - Start here for new developers
2. 📄 README.md - Project overview and quick start
3. 🚢 docs/PRODUCTION_DEPLOYMENT.md - Production deployment
4. 🔒 docs/SECURITY.md - Security guidelines
5. ⚡ docs/guides/MIGRATION_QUICK_REFERENCE.md - Daily commands
6. 🧪 E2E_API_TESTING_PLAN.md - Testing procedures
7. 📖 docs/reference/ - Technical deep dives
```

---

## 🔄 Maintenance Guidelines

### Keeping Structure Clean
1. **Regular Cleanup**: Remove unused files and dependencies
2. **Documentation Updates**: Keep docs in sync with code changes
3. **Test Coverage**: Maintain tests for all critical paths
4. **Dependency Management**: Regular `go mod tidy` and security updates

### Adding New Services
When adding new services, follow this structure:
```
services/new-service/
├── Dockerfile
├── go.mod
├── main.go
├── internal/
│   ├── handlers/
│   ├── services/
│   ├── repositories/
│   └── models/
└── tests/
```

### File Organization Best Practices
- Keep files small and focused (< 500 lines)
- Group related functionality together
- Use meaningful file and package names
- Follow Go conventions for package structure
- Document all public APIs

---

## 📞 Support

If you need help understanding the project structure:

1. **Start with**: [ONBOARDING.md](../ONBOARDING.md)
2. **Questions about deployment**: [docs/PRODUCTION_DEPLOYMENT.md](./PRODUCTION_DEPLOYMENT.md)
3. **Security concerns**: [docs/SECURITY.md](./SECURITY.md)
4. **Daily development**: [docs/guides/MIGRATION_QUICK_REFERENCE.md](./guides/MIGRATION_QUICK_REFERENCE.md)

---

*This structure has evolved through multiple iterations to support scalability, maintainability, and developer productivity. Each design decision has been documented in the [completed-plans](./completed-plans/) directory.*