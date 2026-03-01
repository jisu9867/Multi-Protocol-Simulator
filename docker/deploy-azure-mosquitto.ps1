# Deploy Mosquitto to Azure Container Instances with remote access enabled
# PowerShell script version

$RESOURCE_GROUP = "rg-gateway-dev-korea-01"
$CONTAINER_NAME = "mosquitto-broker"
$DNS_NAME_LABEL = "mosquitto-gateway-$([DateTimeOffset]::Now.ToUnixTimeSeconds())"

Write-Host "Deploying Mosquitto with remote access configuration..." -ForegroundColor Green

# Create command to write config and start mosquitto
# Use single quotes inside double quotes for proper escaping
$commandLine = '/bin/sh -c "mkdir -p /mosquitto/config && echo listener 1883 0.0.0.0 > /mosquitto/config/mosquitto.conf && echo allow_anonymous true >> /mosquitto/config/mosquitto.conf && echo log_dest stdout >> /mosquitto/config/mosquitto.conf && echo log_type all >> /mosquitto/config/mosquitto.conf && echo connection_messages true >> /mosquitto/config/mosquitto.conf && mosquitto -c /mosquitto/config/mosquitto.conf"'

Write-Host "Creating container with Mosquitto broker..." -ForegroundColor Cyan
Write-Host "Command: $commandLine" -ForegroundColor Gray

# Execute az container create command (all on one line to avoid PowerShell issues)
az container create --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME --image eclipse-mosquitto:2.0 --os-type Linux --cpu 1 --memory 1 --ports 1883 8883 --ip-address Public --dns-name-label $DNS_NAME_LABEL --command-line "$commandLine"

if ($LASTEXITCODE -eq 0) {
    Write-Host "`nMosquitto broker deployed successfully!" -ForegroundColor Green
    
    $FQDN = az container show `
      --resource-group $RESOURCE_GROUP `
      --name $CONTAINER_NAME `
      --query ipAddress.fqdn -o tsv
    
    Write-Host "FQDN: $FQDN" -ForegroundColor Cyan
    Write-Host "`nUpdate configs/azure.yaml with this FQDN:" -ForegroundColor Yellow
    Write-Host "  broker: `"$FQDN`:1883`"" -ForegroundColor White
    
    Write-Host "`nWaiting for container to be ready..." -ForegroundColor Yellow
    Start-Sleep -Seconds 10
    
    Write-Host "Checking container logs..." -ForegroundColor Yellow
    az container logs --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME
} else {
    Write-Host "`nFailed to deploy Mosquitto broker. Exit code: $LASTEXITCODE" -ForegroundColor Red
    Write-Host "`nPossible issues:" -ForegroundColor Yellow
    Write-Host "  1. Docker Hub registry temporarily unavailable (retry later)" -ForegroundColor White
    Write-Host "  2. Existing container with same name exists (delete it first)" -ForegroundColor White
    Write-Host "  3. Network connectivity issues" -ForegroundColor White
    Write-Host "`nTo delete existing container:" -ForegroundColor Yellow
    Write-Host "  az container delete --resource-group $RESOURCE_GROUP --name $CONTAINER_NAME --yes" -ForegroundColor White
}

