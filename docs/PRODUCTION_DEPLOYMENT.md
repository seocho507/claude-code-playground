# 🚀 Production Deployment Guide

**Version**: 2.0  
**Last Updated**: 2025-08-27  
**Status**: Production Ready

## 📋 Table of Contents
1. [Pre-Deployment Checklist](#-pre-deployment-checklist)
2. [Infrastructure Requirements](#-infrastructure-requirements)
3. [Deployment Strategies](#-deployment-strategies)
4. [Production Configuration](#-production-configuration)
5. [Database Setup](#-database-setup)
6. [Security Hardening](#-security-hardening)
7. [Monitoring & Observability](#-monitoring--observability)
8. [Backup & Recovery](#-backup--recovery)
9. [Scaling Guidelines](#-scaling-guidelines)
10. [Maintenance Procedures](#-maintenance-procedures)

---

## ✅ Pre-Deployment Checklist

### Code Readiness
- [ ] All tests passing (`go test ./... -v`)
- [ ] No critical security vulnerabilities (`go mod audit`)
- [ ] Code reviewed and approved
- [ ] Documentation updated
- [ ] API version tagged in Git

### Configuration
- [ ] Production environment variables configured
- [ ] SSL/TLS certificates obtained
- [ ] Domain names configured
- [ ] Firewall rules defined
- [ ] Backup strategy implemented

### Infrastructure
- [ ] Database server provisioned
- [ ] Redis cache server ready
- [ ] Load balancer configured
- [ ] Monitoring tools installed
- [ ] Log aggregation setup

---

## 🏗️ Infrastructure Requirements

### Minimum Production Specifications

#### Application Servers
```yaml
CPU: 4 vCPUs
RAM: 8 GB
Storage: 50 GB SSD
Network: 1 Gbps
OS: Ubuntu 22.04 LTS or RHEL 8+
```

#### Database Server (PostgreSQL)
```yaml
CPU: 8 vCPUs
RAM: 16 GB
Storage: 200 GB SSD (with IOPS provisioning)
Network: 10 Gbps (if possible)
PostgreSQL: 15.x
Backup: Daily automated backups
```

#### Redis Cache Server
```yaml
CPU: 2 vCPUs
RAM: 4 GB
Storage: 20 GB SSD
Redis: 7.x
Persistence: AOF enabled
```

### Network Architecture
```
Internet
    ↓
[CDN/CloudFlare]
    ↓
[Load Balancer]
    ↓
[Traefik Gateway]
    ↓
[Auth Service Instances]
    ↓        ↓
[PostgreSQL] [Redis]
```

---

## 🎯 Deployment Strategies

### Option 1: Docker Swarm Deployment

#### Initialize Swarm
```bash
# On manager node
docker swarm init --advertise-addr <MANAGER-IP>

# Add worker nodes
docker swarm join --token <TOKEN> <MANAGER-IP>:2377
```

#### Deploy Stack
```bash
# Create secrets
echo "your_db_password" | docker secret create db_password -
echo "your_jwt_secret" | docker secret create jwt_secret -

# Deploy services
docker stack deploy -c docker-compose.prod.yml auth-stack

# Verify deployment
docker stack services auth-stack
```

### Option 2: Kubernetes Deployment

#### Create Namespace
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: auth-service
```

#### Deploy with Helm
```bash
# Add Helm repository
helm repo add auth-service https://your-repo.com

# Install chart
helm install auth-service auth-service/chart \
  --namespace auth-service \
  --values values.production.yaml

# Check status
kubectl get pods -n auth-service
```

### Option 3: Traditional VM Deployment

#### System Setup
```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | bash

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" \
  -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose
```

#### Deploy Application
```bash
# Clone repository
git clone <repository-url> /opt/auth-service
cd /opt/auth-service

# Setup environment
cp .env.example .env
# Edit .env with production values

# Start services
docker-compose -f docker-compose.prod.yml up -d
```

---

## ⚙️ Production Configuration

### docker-compose.prod.yml
```yaml
version: '3.8'

services:
  auth-service:
    image: auth-service:${VERSION:-latest}
    restart: always
    environment:
      - NODE_ENV=production
      - GIN_MODE=release
    deploy:
      replicas: 3
      resources:
        limits:
          cpus: '2'
          memory: 2G
        reservations:
          cpus: '1'
          memory: 1G
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8001/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  postgres-auth:
    image: postgres:15-alpine
    restart: always
    environment:
      POSTGRES_DB: auth_db
      POSTGRES_USER: ${DB_USER}
      POSTGRES_PASSWORD_FILE: /run/secrets/db_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./backups:/backups
    deploy:
      resources:
        limits:
          cpus: '4'
          memory: 8G
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=2GB
      -c effective_cache_size=6GB
      -c maintenance_work_mem=512MB
      -c checkpoint_completion_target=0.9
      -c wal_buffers=16MB
      -c default_statistics_target=100
      -c random_page_cost=1.1
      -c effective_io_concurrency=200

  redis-cache:
    image: redis:7-alpine
    restart: always
    command: >
      redis-server
      --maxmemory 2gb
      --maxmemory-policy allkeys-lru
      --appendonly yes
      --appendfsync everysec
    volumes:
      - redis_data:/data
    deploy:
      resources:
        limits:
          cpus: '1'
          memory: 2G

  traefik:
    image: traefik:v3.0
    restart: always
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik/acme.json:/acme.json
      - ./traefik/traefik.prod.yml:/etc/traefik/traefik.yml:ro
    deploy:
      placement:
        constraints:
          - node.role == manager

volumes:
  postgres_data:
  redis_data:

secrets:
  db_password:
    external: true
  jwt_secret:
    external: true
```

### Nginx Configuration (Optional)
```nginx
upstream auth_service {
    least_conn;
    server auth-service-1:8001 max_fails=3 fail_timeout=30s;
    server auth-service-2:8001 max_fails=3 fail_timeout=30s;
    server auth-service-3:8001 max_fails=3 fail_timeout=30s;
}

server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    ssl_certificate /etc/ssl/certs/yourdomain.crt;
    ssl_certificate_key /etc/ssl/private/yourdomain.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    location / {
        proxy_pass http://auth_service;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Timeouts
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

---

## 🗄️ Database Setup

### Production Database Configuration

#### 1. Create Production Database
```sql
-- Create database
CREATE DATABASE auth_db_prod;

-- Create user with limited privileges
CREATE USER auth_service WITH ENCRYPTED PASSWORD 'strong_password';
GRANT CONNECT ON DATABASE auth_db_prod TO auth_service;
GRANT USAGE ON SCHEMA public TO auth_service;
GRANT CREATE ON SCHEMA public TO auth_service;

-- Grant table permissions (after migrations)
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO auth_service;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO auth_service;
```

#### 2. Run Migrations
```bash
# Using migration tool
docker-compose exec auth-service /app/migrate migrate

# Verify migrations
docker-compose exec auth-service /app/migrate status
```

#### 3. Database Optimization
```sql
-- Create indexes for performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(refresh_token);

-- Analyze tables
ANALYZE users;
ANALYZE sessions;
ANALYZE user_preferences;
```

---

## 🔒 Security Hardening

### 1. Environment Variables
```bash
# Use strong secrets (generate with openssl)
openssl rand -base64 32  # For JWT secrets
openssl rand -base64 24  # For database passwords
```

### 2. Network Security
```bash
# Configure firewall (UFW example)
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp   # SSH
sudo ufw allow 80/tcp   # HTTP
sudo ufw allow 443/tcp  # HTTPS
sudo ufw allow 5432/tcp # PostgreSQL (only from app servers)
sudo ufw enable
```

### 3. SSL/TLS Configuration
```bash
# Using Let's Encrypt with Traefik
# Traefik automatically handles certificate generation and renewal
```

### 4. Security Headers
```yaml
# In Traefik configuration
http:
  middlewares:
    security-headers:
      headers:
        sslRedirect: true
        stsSeconds: 31536000
        stsIncludeSubdomains: true
        stsPreload: true
        contentTypeNosniff: true
        browserXssFilter: true
        referrerPolicy: "strict-origin-when-cross-origin"
        customFrameOptionsValue: "SAMEORIGIN"
        customResponseHeaders:
          X-Content-Type-Options: "nosniff"
          X-Frame-Options: "DENY"
          X-XSS-Protection: "1; mode=block"
```

### 5. Rate Limiting
```go
// Already implemented in middleware
// Configure via environment variables:
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=60
RATE_LIMIT_BURST_SIZE=10
```

---

## 📊 Monitoring & Observability

### 1. Prometheus Setup
```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'auth-service'
    static_configs:
      - targets: ['auth-service:8001']
    metrics_path: '/metrics'
```

### 2. Grafana Dashboards
```json
{
  "dashboard": {
    "title": "Auth Service Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])"
          }
        ]
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "expr": "rate(http_requests_total{status=~\"5..\"}[5m])"
          }
        ]
      },
      {
        "title": "Response Time",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))"
          }
        ]
      }
    ]
  }
}
```

### 3. Logging with ELK Stack
```yaml
# filebeat.yml
filebeat.inputs:
- type: container
  paths:
    - '/var/lib/docker/containers/*/*.log'

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
```

### 4. Application Performance Monitoring
```bash
# Install APM agent (e.g., New Relic, DataDog)
# Configure via environment variables
APM_ENABLED=true
APM_SERVICE_NAME=auth-service
APM_ENVIRONMENT=production
```

---

## 💾 Backup & Recovery

### Database Backup Strategy

#### Automated Daily Backups
```bash
#!/bin/bash
# backup.sh
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="/backups"
DB_NAME="auth_db_prod"

# Perform backup
pg_dump -h localhost -U postgres -d $DB_NAME | gzip > $BACKUP_DIR/backup_$DATE.sql.gz

# Upload to S3 (optional)
aws s3 cp $BACKUP_DIR/backup_$DATE.sql.gz s3://your-backup-bucket/

# Keep only last 30 days of local backups
find $BACKUP_DIR -name "backup_*.sql.gz" -mtime +30 -delete
```

#### Restore Procedure
```bash
# Restore from backup
gunzip < backup_20250827_120000.sql.gz | psql -h localhost -U postgres -d auth_db_prod

# Verify restoration
psql -h localhost -U postgres -d auth_db_prod -c "SELECT COUNT(*) FROM users;"
```

### Redis Backup
```bash
# Enable AOF persistence
redis-cli CONFIG SET appendonly yes

# Manual backup
redis-cli BGSAVE

# Copy backup files
cp /var/lib/redis/dump.rdb /backups/redis_backup.rdb
```

---

## 📈 Scaling Guidelines

### Horizontal Scaling

#### Add More Application Instances
```bash
# Scale to 5 instances
docker service scale auth-stack_auth-service=5

# Or with docker-compose
docker-compose up -d --scale auth-service=5
```

#### Database Read Replicas
```sql
-- Setup streaming replication
-- On primary
ALTER SYSTEM SET wal_level = replica;
ALTER SYSTEM SET max_wal_senders = 3;
ALTER SYSTEM SET wal_keep_segments = 64;
```

### Vertical Scaling

#### Increase Resources
```yaml
# Update docker-compose.yml
deploy:
  resources:
    limits:
      cpus: '4'    # Increased from 2
      memory: 4G   # Increased from 2G
```

### Caching Strategy
- Implement query caching with Redis
- Use CDN for static assets
- Enable HTTP caching headers

---

## 🔧 Maintenance Procedures

### Rolling Updates
```bash
# Build new image
docker build -t auth-service:v2.1 .

# Update service with zero downtime
docker service update \
  --image auth-service:v2.1 \
  --update-parallelism 1 \
  --update-delay 30s \
  auth-stack_auth-service
```

### Health Checks
```bash
# Check all services
for service in auth-service postgres-auth redis-cache; do
  echo "Checking $service..."
  docker-compose exec $service echo "OK"
done

# Database health
docker-compose exec postgres-auth pg_isready
```

### Log Rotation
```yaml
# Docker logging configuration
logging:
  driver: "json-file"
  options:
    max-size: "100m"
    max-file: "10"
```

### Performance Tuning
```bash
# Monitor resource usage
docker stats

# Check slow queries
docker-compose exec postgres-auth psql -U postgres -c \
  "SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;"

# Redis memory usage
docker-compose exec redis-cache redis-cli INFO memory
```

---

## 📊 Monitoring Alerts

### Critical Alerts
```yaml
- Alert: ServiceDown
  Expression: up{job="auth-service"} == 0
  Duration: 5m
  Severity: critical

- Alert: HighErrorRate
  Expression: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
  Duration: 5m
  Severity: warning

- Alert: DatabaseConnectionFailure
  Expression: pg_up == 0
  Duration: 1m
  Severity: critical

- Alert: HighMemoryUsage
  Expression: container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.9
  Duration: 5m
  Severity: warning
```

---

## 🚀 Launch Checklist

### Final Pre-Launch Steps
- [ ] DNS records configured
- [ ] SSL certificates installed
- [ ] Monitoring dashboards created
- [ ] Alerting rules configured
- [ ] Backup jobs scheduled
- [ ] Load testing completed
- [ ] Security scan passed
- [ ] Documentation published
- [ ] Team trained on procedures
- [ ] Rollback plan documented

### Go-Live Procedure
1. Enable maintenance mode
2. Perform final database backup
3. Deploy application
4. Run smoke tests
5. Monitor metrics for 30 minutes
6. Disable maintenance mode
7. Announce launch completion

---

## 📞 Support Contacts

| Role | Contact | Responsibility |
|------|---------|---------------|
| DevOps Lead | devops@example.com | Infrastructure |
| Database Admin | dba@example.com | Database issues |
| Security Team | security@example.com | Security incidents |
| On-Call Engineer | oncall@example.com | 24/7 support |

---

**Remember**: Always test changes in staging before applying to production!