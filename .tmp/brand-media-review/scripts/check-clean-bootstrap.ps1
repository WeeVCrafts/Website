$ErrorActionPreference = 'Stop'
$taskContainer = 'weevcrafts-clean-bootstrap-check'
if (docker ps -a -q --filter "name=^/$taskContainer$") { throw "Bootstrap-check container already exists; inspect it before retrying." }
$taskKeys = @('DATABASE_URL','WECRATFS_ENV','WECRATFS_LOCAL_MODE','WECRATFS_SEED_EMPTY','WECRATFS_RESET_LOCAL')
$taskSaved = @{}
foreach ($taskKey in $taskKeys) { $taskSaved[$taskKey] = [Environment]::GetEnvironmentVariable($taskKey) }
function Invoke-Bootstrap {
    go run ./cmd/reset-local
    if ($LASTEXITCODE -ne 0) { throw 'Bootstrap command failed' }
}
function Assert-DatabaseValue($taskSql, $taskExpected) {
    $taskValue = docker exec $taskContainer psql -U wecratfs -d wecratfs -At -c $taskSql
    if ($LASTEXITCODE -ne 0 -or $taskValue.Trim() -ne $taskExpected) { throw "Bootstrap assertion failed: $taskSql" }
}
try {
    docker run --rm -d --name $taskContainer -e POSTGRES_USER=wecratfs -e POSTGRES_PASSWORD=wecratfs -e POSTGRES_DB=wecratfs -p '127.0.0.1::5432' postgres:17-alpine | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Failed to start isolated Postgres' }
    $taskReady = $false
    for ($taskAttempt=0; $taskAttempt -lt 30; $taskAttempt++) {
        docker exec $taskContainer pg_isready -U wecratfs -d wecratfs *> $null
        if ($LASTEXITCODE -eq 0) { $taskReady = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $taskReady) { throw 'Isolated Postgres did not become ready' }
    $taskPort = ((docker port $taskContainer 5432/tcp).Trim().Split(':'))[-1]
    $env:DATABASE_URL = "postgres://wecratfs:wecratfs@127.0.0.1:${taskPort}/wecratfs?sslmode=disable"
    $env:WECRATFS_ENV = 'development'
    $env:WECRATFS_LOCAL_MODE = 'true'
    $env:WECRATFS_SEED_EMPTY = 'true'
    $env:WECRATFS_RESET_LOCAL = 'false'
    Invoke-Bootstrap
    Assert-DatabaseValue 'SELECT COUNT(*) FROM users' '3'
    Assert-DatabaseValue 'SELECT COUNT(*) FROM products' '0'
    docker exec $taskContainer psql -U wecratfs -d wecratfs -c "INSERT INTO categories(slug,name) VALUES ('bootstrap-preserve','Preserved category'); INSERT INTO users(email,display_name,status) VALUES ('bootstrap-check@example.test','Preserved account','active')" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Fixture creation failed' }
    Invoke-Bootstrap
    Assert-DatabaseValue 'SELECT COUNT(*) FROM users' '4'
    # Only the isolated test container is truncated, never the Compose database.
    docker exec $taskContainer psql -U wecratfs -d wecratfs -c 'TRUNCATE users CASCADE' *> $null
    if ($LASTEXITCODE -ne 0) { throw 'Isolated user-table cleanup failed' }
    Invoke-Bootstrap
    Assert-DatabaseValue 'SELECT COUNT(*) FROM users' '3'
    Assert-DatabaseValue "SELECT COUNT(*) FROM categories WHERE slug='bootstrap-preserve'" '1'
    Write-Output 'Bootstrap passed: fresh three-account setup, existing accounts preserved, existing categories preserved with an empty user table.'
} finally {
    foreach ($taskKey in $taskKeys) {
        [Environment]::SetEnvironmentVariable($taskKey, $taskSaved[$taskKey])
    }
    docker rm -f $taskContainer *> $null
}
