# ============================================================
#  MySQL 运维平台 - 一键停止脚本
#  作用: 停止后端、Agent、前端三个服务
#  用法: powershell -ExecutionPolicy Bypass -File .\stop.ps1
# ============================================================

# 强制使用 UTF-8 编码，避免中文乱码
$OutputEncoding = [System.Text.Encoding]::UTF8
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
chcp 65001 | Out-Null

# 颜色定义
function Write-Success { param($msg) Write-Host $msg -ForegroundColor Green }
function Write-Info    { param($msg) Write-Host $msg -ForegroundColor Cyan }
function Write-Warn    { param($msg) Write-Host $msg -ForegroundColor Yellow }
function Write-Err     { param($msg) Write-Host $msg -ForegroundColor Red }
function Write-Step    { param($msg) Write-Host "`n=== $msg ===" -ForegroundColor Magenta }

$BackendPort = 8080
$AgentPort   = 9090
$WebPort     = 3000
$ProjectRoot = if ($PSScriptRoot) { (Get-Item $PSScriptRoot).Parent.FullName } else { (Get-Location).Path }
$BackendDir = Join-Path $ProjectRoot "platform-backend"
$AgentDir   = Join-Path $ProjectRoot "agent"
$WebDir     = Join-Path $ProjectRoot "web-console"
$LogDir     = Join-Path $ProjectRoot "logs"
$BackendPidFile = Join-Path $LogDir "backend.pid"
$AgentPidFile   = Join-Path $LogDir "agent.pid"
$WebPidFile     = Join-Path $LogDir "frontend.pid"

# 通过端口查找占用进程
function Get-ProcessByPort {
    param([int]$Port)
    $conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if ($conn) { return ($conn | Select-Object -First 1 -ExpandProperty OwningProcess) }
    return $null
}

# 强制结束进程
function Stop-ProcessSafely {
    param(
        [int]$ProcessId,
        [string]$Name,
        [int]$WaitSec = 5
    )
    $proc = Get-Process -Id $ProcessId -ErrorAction SilentlyContinue
    if (-not $proc) {
        Write-Warn "[警告] $Name (PID: $ProcessId) 已不存在"
        return $true
    }
    try {
        Write-Info "[信息] 正在停止 $Name (PID: $ProcessId)..."
        Stop-Process -Id $ProcessId -Force -ErrorAction Stop
        $elapsed = 0
        while ($elapsed -lt $WaitSec) {
            if (-not (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)) {
                Write-Success "[成功] $Name 已停止"
                return $true
            }
            Start-Sleep -Seconds 1
            $elapsed++
        }
        Write-Err "[错误] $Name 在 $WaitSec 秒内未能停止"
        return $false
    } catch {
        Write-Err "[错误] 停止 $Name 失败: $_"
        return $false
    }
}

function Stop-ProcessFromPidFile {
    param(
        [string]$PidFile,
        [string]$Name
    )
    if (-not (Test-Path -LiteralPath $PidFile)) { return $false }

    $pidText = (Get-Content -LiteralPath $PidFile -ErrorAction SilentlyContinue | Select-Object -First 1).Trim()
    if (-not ($pidText -match '^\d+$')) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
        return $false
    }

    $ok = Stop-ProcessSafely -ProcessId ([int]$pidText) -Name $Name
    if ($ok) {
        Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
    }
    return $ok
}

# 兜底：根据进程名清理（如端口扫描未命中的情况）
function Stop-ProcessByName {
    param([string]$ProcessName, [string]$DisplayName, [string]$RootPath = "")
    $procs = Get-Process -Name $ProcessName -ErrorAction SilentlyContinue
    if (-not $procs) { return }
    Write-Info "[信息] 兜底清理 $DisplayName 进程..."
    foreach ($p in $procs) {
        if ($RootPath -and ($p.Path -notlike "$RootPath*")) { continue }
        try {
            Stop-Process -Id $p.Id -Force -ErrorAction Stop
            Write-Success "[成功] 已结束 $DisplayName (PID: $($p.Id))"
        } catch {
            Write-Warn "[警告] 结束进程失败: $_"
        }
    }
}

# ===================== 主流程 =====================
try {
    Write-Host ""
    Write-Host "  MySQL 运维平台 - 一键停止" -ForegroundColor White -BackgroundColor DarkRed
    Write-Host "  停止时间: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
    Write-Host ""

    $allOk = $true

    # ---------- 1. 停止后端 ----------
    Write-Step "1. 停止后端服务 (端口 $BackendPort)"
    $handled = $false
    if (Stop-ProcessFromPidFile -PidFile $BackendPidFile -Name "后端") {
        $handled = $true
        $targetPid = $null
    } else {
        $targetPid = Get-ProcessByPort -Port $BackendPort
    }
    if ($targetPid) {
        $handled = $true
        if (-not (Stop-ProcessSafely -ProcessId $targetPid -Name "后端")) { $allOk = $false }
    } elseif (-not $handled) {
        Write-Info "[信息] 后端未运行"
    }
    # 兜底：按进程名
    Stop-ProcessByName -ProcessName "platform" -DisplayName "后端进程" -RootPath $BackendDir

    # ---------- 2. 停止 Agent ----------
    Write-Step "2. 停止 Agent 服务 (端口 $AgentPort)"
    $handled = $false
    if (Stop-ProcessFromPidFile -PidFile $AgentPidFile -Name "Agent") {
        $handled = $true
        $targetPid = $null
    } else {
        $targetPid = Get-ProcessByPort -Port $AgentPort
    }
    if ($targetPid) {
        $handled = $true
        if (-not (Stop-ProcessSafely -ProcessId $targetPid -Name "Agent")) { $allOk = $false }
    } elseif (-not $handled) {
        Write-Info "[信息] Agent 未运行"
    }
    # 兜底：按进程名
    Stop-ProcessByName -ProcessName "agent" -DisplayName "Agent 进程" -RootPath $AgentDir

    # ---------- 3. 停止前端 ----------
    Write-Step "3. 停止前端服务 (端口 $WebPort)"
    $handled = $false
    if (Stop-ProcessFromPidFile -PidFile $WebPidFile -Name "前端") {
        $handled = $true
        $targetPid = $null
    } else {
        $targetPid = Get-ProcessByPort -Port $WebPort
    }
    if ($targetPid) {
        $handled = $true
        # 前端是 node 进程 (vite dev server)
        $proc = Get-CimInstance Win32_Process -Filter "ProcessId = $targetPid" -ErrorAction SilentlyContinue
        if ($proc -and $proc.CommandLine -like "*vite*") {
            if (-not (Stop-ProcessSafely -ProcessId $targetPid -Name "前端")) { $allOk = $false }
        } else {
            Write-Warn "[警告] 端口 $WebPort 被非前端进程占用 (PID: $targetPid)，已跳过"
        }
    } elseif (-not $handled) {
        Write-Info "[信息] 前端未运行"
    }

    # 兜底：清理 vite dev 残留
    Get-CimInstance Win32_Process -Filter "Name = 'node.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -like "*vite*bin*vite.js*" -and $_.CommandLine -like "*$WebDir*" } |
        ForEach-Object {
            try {
                Stop-Process -Id $_.ProcessId -Force -ErrorAction Stop
                Write-Success "[成功] 已清理 vite 残留进程 (PID: $($_.ProcessId))"
            } catch {
                Write-Warn "[警告] 清理失败: $_"
            }
        }

    # ---------- 4. 校验端口已释放 ----------
    Write-Step "4. 校验端口状态"
    foreach ($port in $BackendPort, $AgentPort, $WebPort) {
        $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
        $name = switch ($port) {
            8080 { "后端" }
            9090 { "Agent" }
            3000 { "前端" }
        }
        if ($conn) {
            Write-Err "[错误] 端口 $port 仍被占用 (PID: $($conn.OwningProcess))"
            $allOk = $false
        } else {
            Write-Success "[成功] 端口 $port 已释放 ($name)"
        }
    }

    Write-Host ""
    if ($allOk) {
        Write-Host "  ====================== 停止完成 ======================" -ForegroundColor Green
    } else {
        Write-Host "  ================== 停止完成 (有警告) ==================" -ForegroundColor Yellow
    }
    Write-Host ""

    if (-not $allOk) { exit 1 }
    exit 0
}
catch {
    Write-Host ""
    Write-Err "[失败] 停止流程中断: $_"
    exit 1
}
