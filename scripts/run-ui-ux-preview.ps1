param(
    [ValidateSet('baseline', 'preview', 'seed', 'status')]
    [string]$Mode = 'preview'
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path $PSScriptRoot -Parent
$artifactRoot = if ($env:WEEVCRAFTS_UI_ARTIFACTS) { $env:WEEVCRAFTS_UI_ARTIFACTS } else { Join-Path $env:TEMP 'weevcrafts-ui-ux-20261014' }
New-Item -ItemType Directory -Force -Path $artifactRoot | Out-Null

# These endpoints belong exclusively to the named UI QA containers. Never read .env.
$env:DATABASE_URL = 'postgres://weev_ui_qa:weev-ui-qa-local@127.0.0.1:15432/weev_qa_ui_20261014?sslmode=disable'
$env:DATABASE_CA_CERT = ''
$env:REDIS_ADDR = '127.0.0.1:16380'
$env:REDIS_URL = ''
$env:WECRATFS_ENV = 'development'
$env:WECRATFS_LOCAL_MODE = 'true'
$env:WECRATFS_SECURE_COOKIES = 'false'
$env:WECRATFS_PAYMENT_MODE = 'local'
$env:WECRATFS_LOCAL_PAYMENT_SECRET = 'weev-ui-qa-local-payment-only'
$env:WECRATFS_SMTP_ADDR = ''
$env:WECRATFS_RESEND_API_KEY = ''
$env:WECRATFS_RESEND_API_KEY_FILE = ''
$env:RAZORPAY_KEY_ID = ''
$env:RAZORPAY_KEY_SECRET = ''
$env:RAZORPAY_WEBHOOK_SECRET = ''
$env:GOOGLE_APPLICATION_CREDENTIALS = ''
$env:MEILI_ADDR = '127.0.0.1:17770'
$env:S3_BUCKET = 'weevcrafts-ui-qa-20261014'

if ($Mode -eq 'seed') {
    Push-Location $projectRoot
    try {
        go run ./cmd/seed-demo
        if ($LASTEXITCODE -ne 0) { throw 'Isolated UI fixture seed failed.' }
    } finally { Pop-Location }
    exit
}

if ($Mode -eq 'status') {
    docker ps --filter name=weev-ui-qa-20261014 --format '{{.Names}} {{.Status}} {{.Ports}}'
    foreach ($qaPort in @(18080, 18081)) {
        try { Write-Output "$qaPort ready: $((Invoke-WebRequest -UseBasicParsing "http://127.0.0.1:$qaPort/health/ready" -TimeoutSec 5).StatusCode)" }
        catch { Write-Output "$qaPort is not ready" }
    }
    exit
}

$qaPort = if ($Mode -eq 'baseline') { 18081 } else { 18080 }
$env:WECRATFS_ADDR = "127.0.0.1:$qaPort"
$env:WECRATFS_PUBLIC_URL = "http://127.0.0.1:$qaPort"
$binary = Join-Path $artifactRoot "$Mode.exe"
if ($Mode -eq 'preview') {
    Push-Location $projectRoot
    try {
        go build -o $binary ./cmd/website
        if ($LASTEXITCODE -ne 0) { throw 'UI preview build failed.' }
    } finally { Pop-Location }
}
if (-not (Test-Path -LiteralPath $binary)) { throw "Missing UI binary: $binary" }
if (Get-NetTCPConnection -State Listen -LocalPort $qaPort -ErrorAction SilentlyContinue) {
    throw "QA port $qaPort is already in use. Inspect its owner before restarting."
}
$process = Start-Process -FilePath $binary -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru `
    -RedirectStandardOutput (Join-Path $artifactRoot "$Mode.stdout.log") `
    -RedirectStandardError (Join-Path $artifactRoot "$Mode.stderr.log")
Write-Output "Isolated UI $Mode process $($process.Id), http://127.0.0.1:$qaPort"
