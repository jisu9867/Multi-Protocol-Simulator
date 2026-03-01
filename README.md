# Smart Factory Protocol Simulator

스마트팩토리 통합 게이트웨이를 테스트하기 위한 프로토콜 시뮬레이터입니다.

## 개요

이 시뮬레이터는 다양한 프로토콜을 통해 텔레메트리 데이터를 발행하는 도구입니다. 현재는 MQTT 프로토콜을 지원하며, 향후 OPC-UA, Serial, TCP, gRPC, WebSocket, CAN 등 다른 프로토콜을 쉽게 추가할 수 있도록 설계되었습니다.

## 주요 기능

- **다양한 데이터 생성 패턴**: 균등 분포, 정규 분포, 사인파, 계단 함수, 랜덤 워크
- **유연한 속도 제어**: 간격 기반 또는 초당 메시지 수 기반
- **백프레셔 처리**: 큐 기반 버퍼링 및 오버플로우 정책
- **재연결 지원**: 지수 백오프를 사용한 자동 재연결
- **메트릭 수집**: 발행 통계, 실패 횟수, 재연결 횟수 등
- **Graceful Shutdown**: 안전한 종료 및 최종 요약 출력

## 아키텍처

```
┌─────────────┐
│  Generator  │ → TelemetryMessage 생성
└──────┬──────┘
       │
       ↓
┌─────────────┐
│   Engine    │ → Rate Control, Queue Management
└──────┬──────┘
       │
       ↓
┌─────────────┐
│   Adapter   │ → 프로토콜별 발행 (MQTT, OPC-UA, ...)
└─────────────┘
```

### 핵심 컴포넌트

1. **Core**: 프로토콜 독립적인 핵심 로직
   - `TelemetryMessage`: 표준 메시지 모델
   - `Generator`: 데이터 생성 인터페이스 및 구현
   - `Adapter`: 프로토콜 어댑터 인터페이스
   - `Engine`: 실행 엔진 (속도 제어, 큐 관리)

2. **Adapters**: 프로토콜별 구현
   - `mqtt`: MQTT 프로토콜 어댑터

3. **CLI**: 명령줄 인터페이스
   - `run`: 시뮬레이터 실행
   - `validate-config`: 설정 파일 검증
   - `dry-run`: 실제 발행 없이 메시지 생성 테스트

## 설치 및 빌드

### 사전 요구사항

- Go 1.21 이상
- Docker 및 Docker Compose (로컬 MQTT 브로커 실행용)

### 빌드

```bash
# 의존성 다운로드
go mod download

# 빌드
go build -o simulator ./cmd/simulator

# 또는 설치
go install ./cmd/simulator
```

## 사용 방법

### 설정 파일

시뮬레이터는 YAML 형식의 설정 파일을 사용합니다. 예시는 `configs/` 디렉토리에 있습니다.

#### 기본 설정 예시 (`configs/local.yaml`)

```yaml
adapter: mqtt

generator:
  source_id: "sim-001"
  tags:
    - tag: "temp"
      pattern: "uniform"
      min: 20.0
      max: 70.0
      unit: "C"
      quality: "Good"
    
    - tag: "humidity"
      pattern: "normal"
      min: 30.0
      max: 80.0
      mean: 55.0
      stddev: 10.0
      unit: "%"
      quality: "Good"

engine:
  rate_mode: "interval"
  interval_ms: 1000
  jitter_percent: 10.0
  queue_size: 1000
  overflow_policy: "drop_oldest"
  retry_count: 3
  metrics_interval: 10s

mqtt:
  broker: "localhost:1883"
  client_id: "simulator-001"
  topic_template: "factory/{line}/{source_id}/telemetry"
  line: "line-1"
  qos: 1
```

### 실행

#### 로컬 실행

1. **MQTT 브로커 시작** (Docker Compose 사용)

```bash
cd docker
docker-compose up -d mosquitto
```

2. **시뮬레이터 실행**

```bash
# 기본 실행
./simulator run --config ./configs/local.yaml

# 또는 설치된 경우
simulator run --config ./configs/local.yaml
```

3. **설정 검증**

```bash
./simulator validate-config --config ./configs/local.yaml
```

4. **Dry Run** (실제 발행 없이 메시지 생성 테스트)

```bash
./simulator dry-run --config ./configs/local.yaml --count 10
```

#### Azure Container Instances MQTT Broker 사용

Azure Container Instances에 배포된 Mosquitto broker로 데이터를 publish하려면:

1. **Azure MQTT Broker FQDN 확인**

```powershell
# PowerShell
az container show `
  --resource-group rg-gateway-dev-korea-01 `
  --name mosquitto-broker `
  --query ipAddress.fqdn -o tsv
```

```bash
# Bash
az container show \
  --resource-group rg-gateway-dev-korea-01 \
  --name mosquitto-broker \
  --query ipAddress.fqdn -o tsv
```

2. **Azure 설정 파일 업데이트**

`configs/azure.yaml` 파일의 `mqtt.broker` 값을 위에서 확인한 FQDN으로 업데이트합니다:

```yaml
mqtt:
  broker: "mosquitto-gateway-xxxxx.koreacentral.azurecontainer.io:1883"
  # ... 나머지 설정
```

3. **Azure Broker로 시뮬레이터 실행**

```bash
# Azure broker로 실행
./simulator run --config ./configs/azure.yaml --adapter mqtt

# 또는 설치된 경우
simulator run --config ./configs/azure.yaml --adapter mqtt
```

4. **연결 확인**

시뮬레이터가 Azure broker에 연결되면 로그에서 다음과 같은 메시지를 확인할 수 있습니다:

```
[INFO] Connecting to MQTT broker mosquitto-gateway-xxxxx.koreacentral.azurecontainer.io:1883
[INFO] Connected to MQTT broker successfully
[INFO] Publishing telemetry messages...
```

⚠️ **참고**: Azure Container Instances의 Mosquitto broker는 공개 인터넷을 통해 접근 가능하므로 방화벽 규칙을 확인해야 합니다.

#### Docker Compose로 전체 실행

```bash
cd docker
docker-compose up
```

이 명령은 Mosquitto 브로커와 시뮬레이터를 모두 시작합니다.

## 설정 옵션

### Generator 설정

- `source_id`: 소스 식별자
- `tags`: 태그별 설정
  - `tag`: 태그 이름
  - `pattern`: 생성 패턴 (`uniform`, `normal`, `sine`, `step`, `randomwalk`)
  - `min`/`max`: 값 범위
  - `mean`/`stddev`: 정규 분포용 (pattern이 `normal`일 때)
  - `unit`: 단위
  - `quality`: 데이터 품질 (`Good`, `Uncertain`, `Bad`, `Unknown`)

### Engine 설정

- `rate_mode`: 속도 제어 모드 (`interval` 또는 `rate`)
- `interval_ms`: 메시지 간 간격 (밀리초, `rate_mode`가 `interval`일 때)
- `rate`: 초당 메시지 수 (`rate_mode`가 `rate`일 때)
- `jitter_percent`: 간격에 대한 지터 (±%)
- `queue_size`: 큐 크기
- `overflow_policy`: 큐 오버플로우 정책 (`drop_oldest`, `drop_newest`, `retry`)
- `retry_count`: 재시도 횟수
- `metrics_interval`: 메트릭 로깅 간격

### MQTT 설정

- `broker`: 브로커 주소 (`host:port`)
- `client_id`: 클라이언트 ID
- `username`/`password`: 인증 정보 (선택)
- `tls`: TLS 사용 여부
- `keepalive`: Keep-alive 시간 (초)
- `qos`: QoS 레벨 (0, 1, 또는 2)
- `retain`: Retain 플래그
- `topic_template`: 토픽 템플릿 (예: `factory/{line}/{source_id}/telemetry`)
- `line`: 라인 식별자 (토픽 템플릿에서 사용)

## 메시지 형식

시뮬레이터가 발행하는 메시지는 다음 JSON 형식을 따릅니다:

```json
{
  "ts": "2026-01-06T00:00:00+09:00",
  "sourceId": "sim-001",
  "tag": "temp",
  "value": 23.4,
  "unit": "C",
  "quality": "Good",
  "seq": 123,
  "traceId": "abc123def456"
}
```

**중요**: 필드명은 게이트웨이 파서와의 호환성을 위해 위와 동일하게 유지됩니다.

## 메트릭

시뮬레이터는 다음 메트릭을 수집합니다:

- `sent_total`: 발행된 메시지 총 수
- `failed_total`: 실패한 메시지 수
- `reconnect_total`: 재연결 횟수
- `current_rate`: 현재 발행 속도 (msg/s)
- `queue_length`: 현재 큐 길이
- `last_error`: 마지막 오류 메시지
- `run_duration`: 실행 시간 (초)

메트릭은 주기적으로 콘솔에 출력되며, 종료 시 최종 요약이 표시됩니다.

## 테스트

```bash
# 모든 테스트 실행
go test ./...

# 특정 패키지 테스트
go test ./tests/...

# 커버리지 포함
go test -cover ./...
```

## 프로토콜 확장

새로운 프로토콜을 추가하려면:

1. `internal/adapters/<protocol>/` 디렉토리 생성
2. `core.Adapter` 인터페이스 구현
3. 설정 구조체 추가 (`config.go`)
4. `internal/config/config.go`에 설정 로딩 추가
5. `cmd/simulator/main.go`에 어댑터 생성 로직 추가

예시는 `internal/adapters/mqtt/` 디렉토리를 참조하세요.

## 로그 예시

```
[METRICS] sent=100 failed=0 reconnect=0 rate=10.00 msg/s queue=0 duration=10.0s
[METRICS] sent=200 failed=0 reconnect=0 rate=10.00 msg/s queue=0 duration=20.0s
^C
Received signal: interrupt. Shutting down gracefully...

=== Simulation Summary ===
Sent Total:      250
Failed Total:    0
Reconnect Total: 0
Average Rate:    10.00 msg/s
Queue Length:    0
Run Duration:    25.00 seconds
=======================
```

## 문서

- [트러블슈팅 가이드](../docs/TROUBLESHOOTING.md) - 개발 및 통합 과정에서 발생한 주요 이슈와 해결 방법

## 문제 해결

### MQTT 연결 실패

- 브로커가 실행 중인지 확인: `docker ps`
- 브로커 주소와 포트 확인: 설정 파일의 `mqtt.broker`
- 방화벽 설정 확인

### 메시지 발행 실패

- 네트워크 연결 확인
- 브로커 로그 확인: `docker logs mosquitto`
- QoS 설정 확인 (QoS 2는 일부 브로커에서 제한될 수 있음)

## 라이선스

이 프로젝트는 게이트웨이 테스트 목적으로 개발되었습니다.

## 기여

이슈 및 풀 리퀘스트를 환영합니다. 새로운 프로토콜 어댑터 추가는 특히 환영합니다!

