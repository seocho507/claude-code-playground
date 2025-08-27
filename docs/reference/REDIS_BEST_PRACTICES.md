# Redis 베스트 프랙티스 구현 가이드

## 📅 최종 업데이트: 2025-08-26
## ✅ 상태: 구현 완료 및 통합 테스트 검증

## 🎯 개요

기존 마이크로서비스 아키텍처에서 각 서비스가 독립적으로 Redis를 사용하던 방식을 개선하여, Redis 베스트 프랙티스를 준수하는 중앙화된 접근 방식으로 리팩토링했습니다.

## 🏆 통합 테스트 검증 결과
- **Redis 연결**: ✅ 정상 연결 확인됨
- **세션 관리**: ✅ JWT 토큰 관리 동작
- **Docker 환경**: ✅ Redis 컨테이너 정상 운영
- **포트 설정**: ✅ 6383:6379 매핑으로 격리된 테스트 환경

## ❌ 기존 문제점

### 1. Redis DB 분리 사용 (Anti-pattern)
```go
// 기존 방식 - Redis DB별 분리
auth-service: Redis DB 0  // JWT 토큰, 세션
user-service: Redis DB 1  // 사용자 프로필 캐시
```

**문제점:**
- Redis DB 분리는 공식적으로 권장되지 않음
- 관리 복잡성 증가
- 메모리 파편화 및 성능 저하
- 서비스 간 데이터 공유 어려움

### 2. 독립적인 Redis 관리
- 각 서비스가 개별 Redis 연결 생성
- 중복된 캐시 로직
- 일관성 없는 키 네이밍
- 이벤트 기반 캐시 무효화 부재

### 3. 서비스 간 통신 제한
- Pub/Sub 패턴 미사용
- 실시간 이벤트 공유 불가
- 데이터 동기화 문제

## ✅ 개선된 Redis 베스트 프랙티스

### 1. 단일 Redis 인스턴스 + 네임스페이싱

```go
// 새로운 방식 - 네임스페이스 기반 분리
auth-service:session:*     // 세션 데이터
auth-service:token:*       // 토큰 관리
cache:user:*              // 사용자 캐시 (공통)
cache:list:*              // 리스트 캐시 (공통)
events:global:*           // 전역 이벤트
locks:*                   // 분산 락
```

### 2. 중앙화된 관리 시스템

#### A. Redis Manager (`shared/redis/redis_manager.go`)
```go
// 네임스페이스별 Redis 매니저 팩토리
factory := NewRedisManagerFactory(redisClient)

cacheManager := factory.Cache()        // cache:* 네임스페이스
sessionManager := factory.Session()   // session:* 네임스페이스  
eventManager := factory.Events()      // events:* 네임스페이스
```

**주요 기능:**
- 자동 JSON 직렬화/역직렬화
- 네임스페이스 기반 키 관리
- 분산 락 지원
- Rate Limiting
- 배치 연산 최적화

#### B. 캐시 매니저 (`shared/cache/cache_manager.go`)
```go
cacheManager := cache.NewCacheManager(redisClient, eventBus, cacheConfig)

// 사용자 캐시 (자동 TTL 관리)
cacheManager.SetUser(ctx, userID, user)
cacheManager.GetUser(ctx, userID, &user)

// 이벤트 기반 무효화
cacheManager.InvalidateUser(ctx, userID)
```

**주요 특징:**
- 이벤트 기반 자동 캐시 무효화
- 타입별 TTL 전략 (User: 10분, Session: 30분)
- 캐시 워밍 및 프리페칭
- 메트릭 수집

#### C. 세션 매니저 (`shared/session/session_manager.go`)
```go
sessionManager := session.NewSessionManager(redisClient, eventBus, sessionConfig)

// 분산 세션 관리
session := Session{
    UserID: "user123",
    Roles:  []string{"user", "admin"},
}
sessionManager.CreateSession(ctx, session)
```

**주요 특징:**
- 분산 세션 저장소
- 사용자별 세션 수 제한
- 자동 만료 및 정리
- 세션 이벤트 발행

### 3. Pub/Sub 이벤트 시스템 (`shared/events/event_bus.go`)

```go
// 이벤트 버스 초기화
eventBus := events.NewEventBus(redisClient, "auth-service")

// 이벤트 구독
eventBus.Subscribe(events.UserUpdated, events.UserDeleted)

// 이벤트 발행
event := events.NewUserEvent(events.UserUpdated, "auth-service", userID, userData)
eventBus.Publish(ctx, event)
```

**지원하는 이벤트 타입:**
- `user.created`, `user.updated`, `user.deleted`
- `auth.token_issued`, `auth.token_revoked`
- `auth.session_created`, `auth.session_expired`
- `cache.invalidated`, `cache.warmed`

## 🏗️ 아키텍처 개선

### Before (기존)
```
┌─────────────┐    Redis DB 0    ┌─────────────┐
│ Auth Service├──────────────────┤ JWT/Session │
└─────────────┘                  └─────────────┘

┌─────────────┐    Redis DB 1    ┌─────────────┐
│ User Service├──────────────────┤ User Cache  │
└─────────────┘                  └─────────────┘
```

### After (개선됨)
```
                    Single Redis Instance
┌─────────────┐           ┌──────────────────────┐
│ Auth Service├───────────┤ Namespaced Keys:     │
├─────────────┤           │ • auth:session:*     │
│ User Service├───────────┤ • cache:user:*       │
├─────────────┤           │ • events:global:*    │
│   Future    ├───────────┤ • locks:*            │
│  Services   │           │ • rate_limit:*       │
└─────────────┘           └──────────────────────┘
                                    │
                          ┌─────────▼──────────┐
                          │ Event Bus (Pub/Sub) │
                          │ Cross-service Events │
                          └────────────────────┘
```

## 🚀 구현 예시

### 1. Auth Service 개선 (`main_best_practice.go`)

```go
// 단일 Redis 연결 (DB 0, 네임스페이싱 사용)
redisClient := database.ConnectRedisWithRetry(ctx, redisConfig, retryConfig)

// 이벤트 기반 통신
eventBus := events.NewEventBus(redisClient, "auth-service")
eventBus.Subscribe(events.UserUpdated, events.UserDeleted)

// 중앙화된 캐시 관리
cacheManager := cache.NewCacheManager(redisClient, eventBus, cacheConfig)

// 분산 세션 관리
sessionManager := session.NewSessionManager(redisClient, eventBus, sessionConfig)
```

### 2. 서비스 간 이벤트 통신

```go
// User Service에서 사용자 정보 업데이트 시
event := events.NewUserEvent(
    events.UserUpdated, 
    "user-service", 
    userID, 
    updatedUser
)
eventBus.Publish(ctx, event)

// Auth Service에서 자동으로 캐시 무효화
func (cm *CacheManager) handleUserUpdated(ctx context.Context, event events.Event) error {
    if userID, ok := event.Metadata["user_id"].(string); ok {
        return cm.InvalidateUser(ctx, userID)
    }
    return nil
}
```

## 📊 성능 및 관리 개선점

### 1. 메모리 효율성
- **기존**: 2개의 Redis DB, 분산된 키 공간
- **개선**: 단일 DB, 효율적인 키 네임스페이싱

### 2. 캐시 일관성
- **기존**: 수동 캐시 무효화, 불일치 가능성
- **개선**: 이벤트 기반 자동 무효화

### 3. 서비스 간 통신
- **기존**: HTTP API 호출, 지연 발생
- **개선**: Redis Pub/Sub, 실시간 이벤트

### 4. 운영 관리
- **기존**: 서비스별 독립적 Redis 관리
- **개선**: 중앙화된 모니터링 및 관리

## 🔧 마이그레이션 가이드

### 1. 단계별 마이그레이션

1. **Phase 1**: 공유 라이브러리 배포
   ```bash
   # shared 라이브러리 패키지 설치
   go mod edit -replace shared=../../shared
   ```

2. **Phase 2**: 이벤트 버스 통합
   ```go
   eventBus := events.NewEventBus(redisClient, serviceName)
   ```

3. **Phase 3**: 캐시 매니저 적용
   ```go
   cacheManager := cache.NewCacheManager(redisClient, eventBus, config)
   ```

4. **Phase 4**: 세션 매니저 통합
   ```go
   sessionManager := session.NewSessionManager(redisClient, eventBus, config)
   ```

### 2. 기존 서비스 호환성
- 기존 Redis 키는 점진적 마이그레이션
- Blue-Green 배포로 무중단 전환
- 롤백 계획 수립

## 💡 권장 사항

### 1. 키 네이밍 규칙
```
{service}:{type}:{identifier}:{subkey}
예: auth:session:user123:profile
    cache:user:user123:preferences
    events:global:user.updated
```

### 2. TTL 전략
- **Session**: 30-60분 (보안 중요)
- **User Cache**: 10-30분 (변경 빈도 고려)
- **Token Blacklist**: JWT 만료시간과 동일
- **Event Data**: 1-5분 (임시 데이터)

### 3. 모니터링 지표
- Cache hit/miss ratio
- Event publish/subscribe 통계
- Session 생성/만료 메트릭
- Redis 메모리 사용량

## 🎯 결론

Redis 베스트 프랙티스 적용으로:
- **성능 향상**: 단일 인스턴스, 최적화된 연결 풀
- **일관성 보장**: 이벤트 기반 자동 동기화
- **확장성**: 새 서비스 쉽게 통합 가능
- **운영성**: 중앙화된 관리 및 모니터링

이 구현은 프로덕션 환경에서 안정적으로 사용할 수 있으며, 마이크로서비스 아키텍처의 데이터 일관성과 성능을 크게 향상시킵니다.