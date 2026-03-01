# Azure Container Instances에 Mosquitto 배포

Azure Container Instances에 원격 연결을 허용하는 Mosquitto broker를 배포하는 방법입니다.

## 문제 해결: "지정된 경로를 찾을 수 없습니다" 오류

PowerShell에서 `--command-line` 옵션을 사용할 때 따옴표 처리가 문제가 될 수 있습니다. 이를 해결하기 위해 배열을 사용하여 명령을 구성합니다.

## 배포 방법

### 방법 1: PowerShell 스크립트 사용 (권장)

```powershell
cd Multi-Protocol-Simulator\docker
.\deploy-azure-mosquitto.ps1
```

### 방법 2: 직접 명령 실행

```powershell
$RESOURCE_GROUP = "rg-gateway-dev-korea-01"
$CONTAINER_NAME = "mosquitto-broker"
$DNS_NAME_LABEL = "mosquitto-gateway-$([DateTimeOffset]::Now.ToUnixTimeSeconds())"

# 명령을 배열로 구성 (PowerShell 따옴표 문제 해결)
$commandArgs = @(
    '/bin/sh',
    '-c',
    'echo listener 1883 0.0.0.0 > /mosquitto/config/mosquitto.conf && echo allow_anonymous true >> /mosquitto/config/mosquitto.conf && echo log_dest stdout >> /mosquitto/config/mosquitto.conf && echo log_type all >> /mosquitto/config/mosquitto.conf && mosquitto -c /mosquitto/config/mosquitto.conf'
)
$commandLine = $commandArgs -join ' '

# 배포
az container create `
  --resource-group $RESOURCE_GROUP `
  --name $CONTAINER_NAME `
  --image eclipse-mosquitto:2.0 `
  --os-type Linux `
  --cpu 1 `
  --memory 1 `
  --ports 1883 8883 `
  --ip-address Public `
  --dns-name-label $DNS_NAME_LABEL `
  --command-line "$commandLine"
```

## 배포 확인

### 1. FQDN 확인

```powershell
az container show `
  --resource-group rg-gateway-dev-korea-01 `
  --name mosquitto-broker `
  --query ipAddress.fqdn -o tsv
```

### 2. 로그 확인 (리스너가 0.0.0.0에 바인딩되었는지 확인)

```powershell
az container logs `
  --resource-group rg-gateway-dev-korea-01 `
  --name mosquitto-broker
```

성공 시 다음과 같은 로그가 표시됩니다:
```
Opening ipv4 listen socket on 0.0.0.0:1883.
```

### 3. Simulator로 테스트

```powershell
cd Multi-Protocol-Simulator
.\simulator.exe run --config .\configs\azure.yaml --adapter mqtt
```

## 문제 해결

### Docker 레지스트리 오류

```
ERROR: (RegistryErrorResponse) An error response is received from the docker registry 'index.docker.io'
```

**해결 방법:**
- Docker Hub가 일시적으로 사용 불가능할 수 있습니다. 몇 분 후 다시 시도하세요.
- 또는 Azure Container Registry(ACR)를 사용하여 이미지를 호스팅할 수 있습니다.

### 기존 컨테이너가 있는 경우

먼저 기존 컨테이너를 삭제하세요:

```powershell
az container delete `
  --resource-group rg-gateway-dev-korea-01 `
  --name mosquitto-broker `
  --yes
```

### 연결이 계속 끊기는 경우

로그를 확인하여 "Starting in local only mode" 메시지가 있는지 확인하세요. 이 경우 설정 파일이 올바르게 생성되지 않았을 수 있습니다.

```powershell
az container logs `
  --resource-group rg-gateway-dev-korea-01 `
  --name mosquitto-broker
```

"local only mode" 메시지가 있으면 컨테이너를 삭제하고 다시 배포하세요.

