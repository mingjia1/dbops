# ============================================================
#  MySQL 运维平台 - 一键重启脚本
#  作用: 停止后重新启动所有服务
#  用法: powershell -ExecutionPolicy Bypass -File .\restart.ps1
# ============================================================

$OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

$ProjectRoot = if ($PSScriptRoot) { (Get-Item $PSScriptRoot).Parent.FullName } else { (Get-Location).Path }
$stopScript  = Join-Path $ProjectRoot "bin\stop.ps1"
$startScript = Join-Path $ProjectRoot "bin\start.ps1"

# 兼容旧版目录结构: 如果 bin/ 下找不到则尝试 scripts/
if (-not (Test-Path -LiteralPath $stopScript)) {
    $stopScript = Join-Path $ProjectRoot "scripts\stop.ps1"
    $startScript = Join-Path $ProjectRoot "scripts\start.ps1"
}

Write-Host ""
Write-Host "  MySQL 运维平台 - 一键重启" -ForegroundColor White -BackgroundColor DarkMagenta
Write-Host "  重启时间: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
Write-Host ""

if (-not (Test-Path -LiteralPath $stopScript)) {
    Write-Host "[错误] 找不到 stop.ps1: $stopScript" -ForegroundColor Red
    exit 1
}
if (-not (Test-Path -LiteralPath $startScript)) {
    Write-Host "[错误] 找不到 start.ps1: $startScript" -ForegroundColor Red
    exit 1
}

Write-Host "[信息] 正在停止服务..." -ForegroundColor Cyan
& powershell -ExecutionPolicy Bypass -File $stopScript
$stopExit = $LASTEXITCODE

if ($stopExit -ne 0) {
    Write-Host "[警告] 停止阶段返回非零退出码 ($stopExit)，但仍继续启动" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "[信息] 正在启动服务..." -ForegroundColor Cyan
& powershell -ExecutionPolicy Bypass -File $startScript
$startExit = $LASTEXITCODE

exit $startExit
