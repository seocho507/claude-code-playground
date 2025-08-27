# 📜 Scripts Directory

**Purpose**: Production-ready scripts for deployment, testing, and maintenance

## 🚀 Production Scripts

### 1. `setup-git-hooks.sh`
**Purpose**: Install Git pre-commit hooks for schema validation  
**Usage**: `bash scripts/setup-git-hooks.sh`  
**Environment**: Development  
**Critical**: Prevents schema consistency issues during development

### 2. `final-api-test.sh`
**Purpose**: Comprehensive API integration testing  
**Usage**: `bash scripts/final-api-test.sh`  
**Environment**: Testing/CI  
**Output**: `final-api-test-results.txt`

### 3. Docker Scripts (`docker/`)

#### `migration-runner.sh`
**Purpose**: Docker container migration initialization  
**Usage**: Called automatically during Docker startup  
**Environment**: Docker containers  
**Critical**: Ensures Migration-First schema consistency

#### `post-migration-validation.sh`
**Purpose**: Post-migration schema validation  
**Usage**: Called automatically after migrations  
**Environment**: Docker containers  
**Critical**: Validates database integrity after migrations

## 🔧 Script Categories

### Production Essential ✅
- `setup-git-hooks.sh` - Developer workflow protection
- `docker/migration-runner.sh` - Container initialization
- `docker/post-migration-validation.sh` - Schema validation

### Testing & Development 🧪
- `final-api-test.sh` - Integration testing

## 📋 Script Dependencies

All scripts require:
- **Bash 4.0+** or equivalent shell
- **PostgreSQL client tools** (`psql`)
- **curl** for API testing
- **Git** for hook installation

### Environment Variables Required

For Docker scripts:
```bash
POSTGRES_USER=postgres
POSTGRES_DB=auth_db
POSTGRES_PASSWORD=<secure_password>
```

For API testing:
```bash
AUTH_SERVICE_URL=http://localhost:8001
```

## 🚨 Critical Notes

1. **Never modify applied migration scripts** - They maintain referential integrity
2. **Git hooks prevent schema inconsistencies** - Essential for team development
3. **Migration validation is mandatory** - Ensures production readiness
4. **All scripts include comprehensive error handling**

## 🔗 Integration Points

- **Docker Compose**: Calls migration scripts during startup
- **Git Workflow**: Pre-commit hooks run automatically
- **CI/CD**: API tests validate deployment success
- **Production**: Migration validation ensures safe deployments

These scripts form the backbone of the Migration-First approach and are essential for maintaining system reliability.