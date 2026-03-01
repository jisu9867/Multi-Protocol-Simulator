#!/bin/bash
# Deploy Mosquitto to Azure Container Instances with remote access enabled

RESOURCE_GROUP="rg-gateway-dev-korea-01"
CONTAINER_NAME="mosquitto-broker"
DNS_NAME_LABEL="mosquitto-gateway-$(date +%s)"

# Build custom Docker image with configuration
echo "Building custom Mosquitto Docker image..."
docker build -f azure-mosquitto.Dockerfile -t azure-mosquitto:latest .

# Push to Azure Container Registry (requires ACR setup)
# For simplicity, we'll use Docker Hub or Azure Container Registry
# If using Azure Container Registry:
# ACR_NAME="your-acr-name"
# az acr login --name $ACR_NAME
# docker tag azure-mosquitto:latest $ACR_NAME.azurecr.io/azure-mosquitto:latest
# docker push $ACR_NAME.azurecr.io/azure-mosquitto:latest

echo "Deploying to Azure Container Instances..."
az container create \
  --resource-group $RESOURCE_GROUP \
  --name $CONTAINER_NAME \
  --image azure-mosquitto:latest \
  --os-type Linux \
  --cpu 1 \
  --memory 1 \
  --ports 1883 8883 \
  --ip-address Public \
  --dns-name-label $DNS_NAME_LABEL \
  --registry-login-server docker.io \
  --registry-username $DOCKER_USERNAME \
  --registry-password $DOCKER_PASSWORD

# Get FQDN
echo "Getting FQDN..."
FQDN=$(az container show \
  --resource-group $RESOURCE_GROUP \
  --name $CONTAINER_NAME \
  --query ipAddress.fqdn -o tsv)

echo "Mosquitto broker deployed successfully!"
echo "FQDN: $FQDN"
echo "Update configs/azure.yaml with this FQDN"

