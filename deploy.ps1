# #region Build & Deploy
$ErrorActionPreference = "Stop"

Write-Host "[Deploy-Main] Starting build and deployment for AWS Lambda..."

# 1. Cross-compile for AWS Lambda Linux ARM64
$env:GOOS = "linux"
$env:GOARCH = "arm64"
$env:CGO_ENABLED = "0"

if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}

Write-Host "[Deploy-Shortener] Compiling shortener service..."
go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/shortener
Compress-Archive -Path bootstrap -DestinationPath bin/shortener.zip -Force
Remove-Item -Path bootstrap -Force

Write-Host "[Deploy-Analytics] Compiling analytics service..."
go build -trimpath -ldflags="-s -w" -o bootstrap ./cmd/analytics
Compress-Archive -Path bootstrap -DestinationPath bin/analytics.zip -Force
Remove-Item -Path bootstrap -Force

# 2. Terraform Apply
Write-Host "[Deploy-Terraform] Applying changes with Terraform..."
terraform -chdir=infra_tf apply -auto-approve

# 3. Output results
$apiEndpoint = (terraform -chdir=infra_tf output -raw api_endpoint).TrimEnd('/')
Write-Host "[Deploy-Main] Deployment successful. API Endpoint: $apiEndpoint"
# #endregion
