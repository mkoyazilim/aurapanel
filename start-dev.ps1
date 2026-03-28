param(
  [string]$ProjectRoot = "D:\Projeler\aurapanel"
)

$ErrorActionPreference = "Stop"

function Write-Info([string]$Message) {
  Write-Host "[INFO] $Message" -ForegroundColor Cyan
}

function Write-WarnLine([string]$Message) {
  Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Write-ErrorLine([string]$Message) {
  Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Test-Http([string]$Url, [int]$TimeoutSec = 2) {
  try {
    $resp = Invoke-WebRequest -UseBasicParsing -Uri $Url -TimeoutSec $TimeoutSec
    return ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500)
  } catch {
    return $false
  }
}

function Wait-Http([string]$Url, [int]$Retries = 25, [int]$SleepSec = 1) {
  for ($i = 0; $i -lt $Retries; $i++) {
    if (Test-Http -Url $Url -TimeoutSec 2) {
      return $true
    }
    Start-Sleep -Seconds $SleepSec
  }
  return $false
}

function Start-DevShell([string]$Title, [string]$Command) {
  Start-Process powershell -ArgumentList "-NoExit", "-Command", "`$host.UI.RawUI.WindowTitle = '$Title'; $Command" | Out-Null
}

$service = Join-Path $ProjectRoot "panel-service"
$gateway = Join-Path $ProjectRoot "api-gateway"
$frontend = Join-Path $ProjectRoot "frontend"

if (!(Test-Path $service) -or !(Test-Path $gateway) -or !(Test-Path $frontend)) {
  Write-ErrorLine "Project folders not found under: $ProjectRoot"
  exit 1
}

$goCmd = Get-Command go -ErrorAction SilentlyContinue
$serviceExe = Join-Path $service "panel-service.exe"
$gatewayExe = Join-Path $gateway "apigw.exe"
$npmCmd = Get-Command npm -ErrorAction SilentlyContinue
$sharedJwtSecret = "aurapanel_dev_only_secret_change_me"
$devAdminEmail = "admin@server.com"
$devAdminPassword = "password123"
$serviceUrl = "http://127.0.0.1:8081"

Write-Info "Starting AuraPanel Panel Service on :8081 ..."
if ($goCmd) {
  Start-DevShell -Title "AuraPanel Panel Service" -Command "`$env:AURAPANEL_DEV_SIMULATION='1'; `$env:AURAPANEL_JWT_SECRET='$sharedJwtSecret'; `$env:AURAPANEL_ADMIN_EMAIL='$devAdminEmail'; `$env:AURAPANEL_ADMIN_PASSWORD='$devAdminPassword'; Set-Location '$service'; & '$($goCmd.Source)' run ."
} elseif (Test-Path $serviceExe) {
  Start-DevShell -Title "AuraPanel Panel Service" -Command "`$env:AURAPANEL_DEV_SIMULATION='1'; `$env:AURAPANEL_JWT_SECRET='$sharedJwtSecret'; `$env:AURAPANEL_ADMIN_EMAIL='$devAdminEmail'; `$env:AURAPANEL_ADMIN_PASSWORD='$devAdminPassword'; Set-Location '$service'; & '$serviceExe'"
} else {
  Write-ErrorLine "Go runtime not found and compiled service binary missing: $serviceExe"
  exit 1
}

Write-Info "Starting API Gateway on :8090 ..."
if ($goCmd) {
  Start-DevShell -Title "AuraPanel Gateway" -Command "`$env:AURAPANEL_DEV_SIMULATION='1'; `$env:AURAPANEL_JWT_SECRET='$sharedJwtSecret'; `$env:AURAPANEL_ADMIN_EMAIL='$devAdminEmail'; `$env:AURAPANEL_ADMIN_PASSWORD='$devAdminPassword'; `$env:AURAPANEL_SERVICE_URL='$serviceUrl'; Set-Location '$gateway'; & '$($goCmd.Source)' run ."
} elseif (Test-Path $gatewayExe) {
  Start-DevShell -Title "AuraPanel Gateway" -Command "`$env:AURAPANEL_DEV_SIMULATION='1'; `$env:AURAPANEL_JWT_SECRET='$sharedJwtSecret'; `$env:AURAPANEL_ADMIN_EMAIL='$devAdminEmail'; `$env:AURAPANEL_ADMIN_PASSWORD='$devAdminPassword'; `$env:AURAPANEL_SERVICE_URL='$serviceUrl'; Set-Location '$gateway'; & '$gatewayExe'"
} else {
  Write-WarnLine "Go runtime and gateway binary not found. API calls will fail."
}

Write-Info "Starting Frontend on :5173 ..."
if ($npmCmd) {
  Start-DevShell -Title "AuraPanel Frontend" -Command "Set-Location '$frontend'; npm run dev"
} else {
  Write-ErrorLine "npm command not found. Frontend cannot be started."
  exit 1
}

Write-Info "Waiting for services to become ready ..."
$serviceOk = Wait-Http -Url "http://127.0.0.1:8081/api/v1/health" -Retries 30 -SleepSec 1
$gatewayOk = Wait-Http -Url "http://127.0.0.1:8090/api/health" -Retries 30 -SleepSec 1
$frontendOk = Wait-Http -Url "http://127.0.0.1:5173" -Retries 30 -SleepSec 1

if ($serviceOk) {
  Write-Host "[OK] Panel service is reachable: http://127.0.0.1:8081/api/v1/health" -ForegroundColor Green
} else {
  Write-WarnLine "Panel service health endpoint did not respond in time."
}

if ($gatewayOk) {
  Write-Host "[OK] Gateway is reachable: http://127.0.0.1:8090/api/health" -ForegroundColor Green
} else {
  Write-WarnLine "Gateway health endpoint did not respond in time."
}

if ($frontendOk) {
  Write-Host "[OK] Frontend is reachable: http://127.0.0.1:5173" -ForegroundColor Green
} else {
  Write-WarnLine "Frontend dev server did not respond in time."
}

Write-Host ""
Write-Host "AuraPanel dev environment launched." -ForegroundColor Green
Write-Host "Service:  http://127.0.0.1:8081"
Write-Host "Gateway:  http://127.0.0.1:8090"
Write-Host "Frontend: http://127.0.0.1:5173"
Write-Host "Login:    $devAdminEmail / $devAdminPassword"
