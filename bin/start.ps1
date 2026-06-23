# ============================================================
#  MySQL 运维平台 - 一键启动脚本 (含自动重新编译)
#  作用: 编译后端/Agent/前端, 启动三个服务
#  用法: powershell -ExecutionPolicy Bypass -File .\start.ps1
#  选项: -SkipBuild    跳过自动编译, 直接启动 (使用已存在的产物)
#        -Component    仅启动指定组件: backend / agent / frontend (默认启动全部)
# ============================================================

[CmdletBinding()]
param(
    [switch]$SkipBuild = $false,
    [ValidateSet("backend", "agent", "frontend", IgnoreCase = $true)]
    [string]$Component = ""
)

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

function Normalize-ProcessPathEnvironment {
    # Windows 上 $env:PATH 与 $env:Path 大小写歧义会导致 cmd /c 调用时丢失 PATH
    # 保留当前 Process PATH, 同时设置 Path 和 PATH 两个键确保 cmd /c 能看到
    $pathValue = $env:PATH
    if ([string]::IsNullOrEmpty($pathValue)) {
        $pathValue = $env:Path
    }
    if (-not [string]::IsNullOrEmpty($pathValue)) {
        [Environment]::SetEnvironmentVariable("Path", $pathValue, "Process")
        [Environment]::SetEnvironmentVariable("PATH", $pathValue, "Process")
    }
}

Normalize-ProcessPathEnvironment

# 路径定义
$ProjectRoot = if ($PSScriptRoot) { (Get-Item $PSScriptRoot).Parent.FullName } else { (Get-Location).Path }
$GoRoot      = "D:\Program Files\go"
$BackendDir  = Join-Path $ProjectRoot "backend"
$AgentDir    = Join-Path $ProjectRoot "agent"
$WebDir      = Join-Path $ProjectRoot "frontend"
$LogDir      = Join-Path $ProjectRoot "logs"

$BackendExe  = Join-Path $BackendDir "bin\platform.exe"
$AgentExe    = Join-Path $AgentDir "bin\agent.exe"
$BackendPidFile = Join-Path $LogDir "backend.pid"
$AgentPidFile   = Join-Path $LogDir "agent.pid"
$WebPidFile     = Join-Path $LogDir "frontend.pid"

$BackendPort = 8080
$AgentPort   = 9090
$WebPort     = 3000

$StartAll = ($Component -eq "")

# 错误处理封装：成功返回 0，失败抛出并打印中文消息
function Assert-Exists {
    param([string]$Path, [string]$Desc)
    if (-not (Test-Path -LiteralPath $Path)) {
        Write-Err "[错误] 未找到 $Desc"
        Write-Err "       路径: $Path"
        throw "缺少必要文件: $Path"
    }
}

function Assert-Running {
    param([int]$Port, [string]$Name)
    $conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if ($conn) {
        $pidText = (($conn | Select-Object -ExpandProperty OwningProcess -Unique) -join ",")
        Write-Warn "[警告] $Name 已在端口 $Port 上运行 (PID: $pidText)"
        Write-Warn "       如需重启，请先运行 stop.ps1"
        return ($conn | Select-Object -First 1 -ExpandProperty OwningProcess)
    }
    return 0
}

function Start-Background {
    param(
        [string]$FilePath,
        [string]$WorkingDir,
        [string]$LogOut,
        [string]$LogErr,
        [hashtable]$Env = $null
    )
    $startArgs = @{
        FilePath               = $FilePath
        WorkingDirectory       = $WorkingDir
        RedirectStandardOutput = $LogOut
        RedirectStandardError  = $LogErr
        PassThru               = $true
        WindowStyle            = 'Hidden'
    }
    if (-not $Env -or $Env.Count -eq 0) {
        return Start-Process @startArgs
    }

    $oldValues = @{}
    foreach ($key in $Env.Keys) {
        $oldValues[$key] = [Environment]::GetEnvironmentVariable($key, "Process")
        [Environment]::SetEnvironmentVariable($key, [string]$Env[$key], "Process")
    }
    try {
        return Start-Process @startArgs
    } finally {
        foreach ($key in $Env.Keys) {
            [Environment]::SetEnvironmentVariable($key, $oldValues[$key], "Process")
        }
    }
}

function Wait-PortReady {
    param(
        [int]$Port,
        [string]$Name,
        [int]$TimeoutSec = 60,
        [System.Diagnostics.Process]$Process = $null,
        [string]$HealthUrl = ""
    )
    $elapsed = 0
    while ($elapsed -lt $TimeoutSec) {
        if ($Process) {
            $Process.Refresh()
            if ($Process.HasExited) {
                throw "$Name 进程在端口 $Port 就绪前已退出 (退出码: $($Process.ExitCode))"
            }
        }
        if ($HealthUrl) {
            try {
                $response = Invoke-WebRequest -Uri $HealthUrl -UseBasicParsing -TimeoutSec 2
                if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 500) {
                    return $true
                }
            } catch { }
        }
        $conn = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
        if ($conn) {
            return $true
        }
        Start-Sleep -Seconds 1
        $elapsed++
    }
    return $false
}

# 判断源码是否有更新（任一 go 文件比 exe 新）
function Test-NeedRebuild {
    param(
        [string]$SourceDir,
        [string]$ExePath
    )
    if (-not (Test-Path -LiteralPath $ExePath)) { return $true }
    $exeTime = (Get-Item -LiteralPath $ExePath).LastWriteTimeUtc
    $newerFiles = Get-ChildItem -Path $SourceDir -Recurse -Include *.go -ErrorAction SilentlyContinue |
        Where-Object { $_.LastWriteTimeUtc -gt $exeTime }
    return ($null -ne $newerFiles -and $newerFiles.Count -gt 0)
}

# 通用构建步骤：执行命令并实时输出
function Invoke-Build {
    param(
        [string]$Title,
        [string]$WorkingDir,
        [string]$Command
    )
    Write-Step $Title
    Write-Info "[信息] 工作目录: $WorkingDir"
    Write-Info "[信息] 执行命令: $Command"

    $prevLocation = Get-Location
    Set-Location -LiteralPath $WorkingDir
    try {
        # 通过 cmd /c 运行, 实时输出
        cmd /c $Command
        $exitCode = $LASTEXITCODE
    } finally {
        Set-Location -LiteralPath $prevLocation
    }

    if ($exitCode -ne 0) {
        Write-Err "[错误] $Title 失败 (退出码 $exitCode)"
        throw "$Title 失败"
    }

    Write-Success "[成功] $Title 完成"
    return $true
}

# ===================== 主流程 =====================
try {
    Write-Host ""
    Write-Host "  MySQL 运维平台 - 启动脚本" -ForegroundColor White -BackgroundColor DarkBlue
    Write-Host "  启动时间: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Gray
    if ($SkipBuild) {
        Write-Host "  模式:    跳过编译, 直接启动已有产物" -ForegroundColor Yellow
    } else {
        Write-Host "  模式:    自动重新编译, 确保运行最新代码" -ForegroundColor Cyan
    }
    if (-not $StartAll) {
        Write-Host "  组件:    仅启动 $Component" -ForegroundColor Green
    } else {
        Write-Host "  组件:    启动全部服务" -ForegroundColor Green
    }
    Write-Host ""

    # ---------- 1. 检查 Go 环境 ----------
    if ($StartAll -or $Component -eq "backend" -or $Component -eq "agent") {
        Write-Step "1. 检查 Go 环境"
        $GoExe = Join-Path $GoRoot "bin\go.exe"
        if (-not (Test-Path -LiteralPath $GoExe)) {
            Write-Err "[错误] 未检测到 Go 环境"
            Write-Err "       预期路径: $GoExe"
            Write-Err "       请确认 Go 已安装到 D:\Program Files\go"
            Write-Info "[提示] 可访问 https://golang.google.cn/dl/ 下载并解压到上述路径"
            throw "Go 环境缺失"
        }
        $goVer = & $GoExe version
        Write-Success "[成功] Go 已安装: $goVer"
    }

    # ---------- 2. 检查 Node.js ----------
    if ($StartAll -or $Component -eq "frontend") {
        Write-Step "2. 检查 Node.js 环境"
        $nodeVer = $null
        try {
            $nodeVer = node --version 2>$null
        } catch { }
        if (-not $nodeVer) {
            Write-Err "[错误] 未检测到 Node.js"
            Write-Err "       请安装 Node.js 18.x 及以上版本: https://nodejs.org/"
            throw "Node.js 缺失"
        }
        Write-Success "[成功] Node.js 已安装: $nodeVer"
    }

    # ---------- 3. 检查项目目录 ----------
    Write-Step "3. 检查项目目录"
    if ($StartAll -or $Component -eq "backend") {
        if (-not (Test-Path -LiteralPath $BackendDir)) { throw "找不到后端目录: $BackendDir" }
    }
    if ($StartAll -or $Component -eq "agent") {
        if (-not (Test-Path -LiteralPath $AgentDir))   { throw "找不到 Agent 目录: $AgentDir" }
    }
    if ($StartAll -or $Component -eq "frontend") {
        if (-not (Test-Path -LiteralPath $WebDir))     { throw "找不到前端目录: $WebDir" }
    }
    Write-Success "[成功] 项目目录齐全"

    # ---------- 4. 准备构建产物目录 ----------
    Write-Step "4. 准备构建产物目录"
    $BackendBinDir = Join-Path $BackendDir "bin"
    $AgentBinDir   = Join-Path $AgentDir "bin"
    $WebDistDir    = Join-Path $WebDir "dist"
    $WebNodeModulesDir = Join-Path $WebDir "node_modules"
    if (-not (Test-Path -LiteralPath $BackendBinDir)) { New-Item -ItemType Directory -Path $BackendBinDir -Force | Out-Null }
    if (-not (Test-Path -LiteralPath $AgentBinDir))   { New-Item -ItemType Directory -Path $AgentBinDir   -Force | Out-Null }
    Write-Success "[成功] 构建目录就绪"

    # ---------- 5. 重新编译后端 ----------
    if (-not $SkipBuild -and ($StartAll -or $Component -eq "backend")) {
        $needBackend = Test-NeedRebuild -SourceDir $BackendDir -ExePath $BackendExe
        if ($needBackend) {
            Invoke-Build -Title "5. 编译后端 (Go)" -WorkingDir $BackendDir `
                -Command "go build -o bin\platform.exe .\cmd\main.go"
        } else {
            Write-Step "5. 编译后端 (Go) - 跳过 (源码未变化)"
            Write-Info "[信息] 现有产物已是最新: $BackendExe"
        }
    } elseif (-not $StartAll -and $Component -ne "backend") {
        Write-Step "5. 编译后端 (Go) - 跳过 (未选择此组件)"
    } else {
        Write-Step "5. 编译后端 (Go) - 跳过 (-SkipBuild)"
    }

    # ---------- 6. 重新编译 Agent ----------
    if (-not $SkipBuild -and ($StartAll -or $Component -eq "agent")) {
        $needAgent = Test-NeedRebuild -SourceDir $AgentDir -ExePath $AgentExe
        if ($needAgent) {
            Invoke-Build -Title "6. 编译 Agent (Go)" -WorkingDir $AgentDir `
                -Command "go build -o bin\agent.exe .\cmd\main.go"
        } else {
            Write-Step "6. 编译 Agent (Go) - 跳过 (源码未变化)"
            Write-Info "[信息] 现有产物已是最新: $AgentExe"
        }
    } elseif (-not $StartAll -and $Component -ne "agent") {
        Write-Step "6. 编译 Agent (Go) - 跳过 (未选择此组件)"
    } else {
        Write-Step "6. 编译 Agent (Go) - 跳过 (-SkipBuild)"
    }

    # ---------- 7. 重新构建前端 ----------
    if (-not $SkipBuild -and ($StartAll -or $Component -eq "frontend")) {
        if (-not (Test-Path -LiteralPath $WebNodeModulesDir)) {
            Write-Step "7. 安装前端依赖 (npm install)"
            Write-Info "[信息] 检测到 node_modules 缺失, 先安装依赖..."
            $prev = Get-Location
            Set-Location -LiteralPath $WebDir
            try {
                cmd /c "npm install"
                if ($LASTEXITCODE -ne 0) {
                    throw "npm install 失败"
                }
            } finally {
                Set-Location -LiteralPath $prev
            }
            Write-Success "[成功] npm install 完成"
        }

        $needWeb = $true
        if (Test-Path -LiteralPath $WebDistDir) {
            $distTime = (Get-Item -LiteralPath $WebDistDir).LastWriteTimeUtc
            $newerSrc = Get-ChildItem -Path $WebDir -Recurse -Include *.ts,*.tsx,*.js,*.jsx,*.json,*.css -ErrorAction SilentlyContinue |
                Where-Object {
                    $_.FullName -notmatch '[\\/](node_modules|dist|coverage)[\\/]' -and
                    $_.LastWriteTimeUtc -gt $distTime
                }
            if (-not $newerSrc) { $needWeb = $false }
        }

        if ($needWeb) {
            Invoke-Build -Title "7. 构建前端 (vite build)" -WorkingDir $WebDir `
                -Command "npx vite build"
        } else {
            Write-Step "7. 构建前端 (vite build) - 跳过 (源码未变化)"
            Write-Info "[信息] 现有 dist 已是最新"
        }
    } elseif (-not $StartAll -and $Component -ne "frontend") {
        Write-Step "7. 构建前端 (vite build) - 跳过 (未选择此组件)"
    } else {
        Write-Step "7. 构建前端 (vite build) - 跳过 (-SkipBuild)"
    }

    # ---------- 8. 验证构建产物 ----------
    Write-Step "8. 验证构建产物"
    if ($StartAll -or $Component -eq "backend") {
        Assert-Exists $BackendExe "后端可执行文件"
    }
    if ($StartAll -or $Component -eq "agent") {
        Assert-Exists $AgentExe   "Agent 可执行文件"
    }
    if ($StartAll -or $Component -eq "frontend") {
        Assert-Exists $WebDistDir "前端构建产物"
        $webIndex = Join-Path $WebDistDir "index.html"
        Assert-Exists $webIndex   "前端 index.html"
    }
    Write-Success "[成功] 构建产物齐全"

    # ---------- 9. 准备日志目录 ----------
    Write-Step "9. 准备日志目录"
    if (-not (Test-Path -LiteralPath $LogDir)) {
        New-Item -ItemType Directory -Path $LogDir -Force | Out-Null
    }
    Write-Success "[成功] 日志目录: $LogDir"

    # ---------- 9.5. 加载 .env (DBOPS_* 必填) ----------
    Write-Step "9.5. 加载环境变量 (.env)"
    $repoRoot = $ProjectRoot
    if (-not $repoRoot) { $repoRoot = (Get-Location).Path }
    $envFile = Join-Path $repoRoot ".env"
    $envMap = @{}
    $requiredEnv = @("DBOPS_DB_URL", "DBOPS_JWT_SECRET", "DBOPS_ENCRYPTION_KEY", "DBOPS_AGENT_TOKEN")
    if (Test-Path -LiteralPath $envFile) {
        Get-Content $envFile | ForEach-Object {
            $line = $_.Trim()
            if ($line -and -not $line.StartsWith("#") -and $line -match "^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.*)$") {
                $k = $matches[1]; $v = $matches[2]
                $envMap[$k] = $v
                Set-Item -Path "Env:$k" -Value $v
            }
        }
        Write-Success "[成功] 已加载 .env ($($envFile))"
    } else {
        Write-Warn "[警告] 未找到 .env, 启动会 fail-fast 提示. 复制 .env.example 到 .env 并填 4 个 DBOPS_* 变量."
    }
    foreach ($key in $requiredEnv) {
        if ([string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($key, "Process"))) {
            Write-Warn "[警告] 缺 env $key, backend 启动会 fail-fast"
        }
    }

    # ---------- 10. 启动后端 ----------
    if ($StartAll -or $Component -eq "backend") {
        Write-Step "10. 启动后端服务 (端口 $BackendPort)"
        $alreadyBackend = Assert-Running -Port $BackendPort -Name "后端"
        if ($alreadyBackend) {
            Set-Content -LiteralPath $BackendPidFile -Value $alreadyBackend -Encoding ASCII
        } else {
            $proc = Start-Background `
                -FilePath $BackendExe `
                -WorkingDir $BackendDir `
                -LogOut (Join-Path $LogDir "backend.log") `
                -LogErr (Join-Path $LogDir "backend.err") `
                -Env $envMap
            Set-Content -LiteralPath $BackendPidFile -Value $proc.Id -Encoding ASCII
            Write-Info "[信息] 后端进程已拉起 (PID: $($proc.Id))"
            if (Wait-PortReady -Port $BackendPort -Name "后端" -TimeoutSec 60 -Process $proc -HealthUrl "http://localhost:$BackendPort/health") {
                Write-Success "[成功] 后端服务已就绪: http://localhost:$BackendPort"
            } else {
                Write-Err "[错误] 后端服务在 60 秒内未就绪"
                Write-Err "       请查看日志: $LogDir\backend.err"
                throw "后端启动超时"
            }
        }
    }

    # ---------- 11. 启动 Agent ----------
    if ($StartAll -or $Component -eq "agent") {
        Write-Step "11. 启动 Agent 服务 (端口 $AgentPort)"
        $alreadyAgent = Assert-Running -Port $AgentPort -Name "Agent"
        if ($alreadyAgent) {
            Set-Content -LiteralPath $AgentPidFile -Value $alreadyAgent -Encoding ASCII
        } else {
            $proc = Start-Background `
                -FilePath $AgentExe `
                -WorkingDir $AgentDir `
                -LogOut (Join-Path $LogDir "agent.log") `
                -LogErr (Join-Path $LogDir "agent.err") `
                -Env $envMap
            Set-Content -LiteralPath $AgentPidFile -Value $proc.Id -Encoding ASCII
            Write-Info "[信息] Agent 进程已拉起 (PID: $($proc.Id))"
            if (Wait-PortReady -Port $AgentPort -Name "Agent" -TimeoutSec 60 -Process $proc -HealthUrl "http://localhost:$AgentPort/health") {
                Write-Success "[成功] Agent 服务已就绪: http://localhost:$AgentPort"
            } else {
                Write-Err "[错误] Agent 服务在 60 秒内未就绪"
                Write-Err "       请查看日志: $LogDir\agent.err"
                throw "Agent 启动超时"
            }
        }
    }

    # ---------- 12. 启动前端 ----------
    if ($StartAll -or $Component -eq "frontend") {
        Write-Step "12. 启动前端服务 (端口 $WebPort)"
        $alreadyWeb = Assert-Running -Port $WebPort -Name "前端"
        if ($alreadyWeb) {
            Set-Content -LiteralPath $WebPidFile -Value $alreadyWeb -Encoding ASCII
        } else {
            $viteScript = Join-Path $WebDir "node_modules\vite\bin\vite.js"
            Assert-Exists $viteScript "vite 启动脚本"
            $proc = Start-Process `
                -FilePath "node" `
                -ArgumentList "$viteScript --port $WebPort --host" `
                -WorkingDirectory $WebDir `
                -RedirectStandardOutput (Join-Path $LogDir "web.log") `
                -RedirectStandardError (Join-Path $LogDir "web.err") `
                -PassThru `
                -WindowStyle Hidden
            Set-Content -LiteralPath $WebPidFile -Value $proc.Id -Encoding ASCII
            Write-Info "[信息] 前端进程已拉起 (PID: $($proc.Id))"
            if (Wait-PortReady -Port $WebPort -Name "前端" -TimeoutSec 60 -Process $proc -HealthUrl "http://localhost:$WebPort/") {
                Write-Success "[成功] 前端服务已就绪: http://localhost:$WebPort"
            } else {
                Write-Err "[错误] 前端服务在 60 秒内未就绪"
                Write-Err "       请查看日志: $LogDir\web.err"
                throw "前端启动超时"
            }
        }
    }

    # ---------- 13. 健康检查 ----------
    Write-Step "13. 健康检查"
    $endpoints = @()
    if ($StartAll -or $Component -eq "backend") {
        $endpoints += @{ Url = "http://localhost:$BackendPort/health"; Name = "后端健康检查" }
    }
    if ($StartAll -or $Component -eq "agent") {
        $endpoints += @{ Url = "http://localhost:$AgentPort/health";   Name = "Agent 健康检查" }
    }
    if ($StartAll -or $Component -eq "frontend") {
        $endpoints += @{ Url = "http://localhost:$WebPort/";          Name = "前端首页" }
    }
    $allOk = $true
    foreach ($ep in $endpoints) {
        try {
            $r = Invoke-WebRequest -Uri $ep.Url -UseBasicParsing -TimeoutSec 5
            Write-Success "[成功] $($ep.Name): $($r.StatusCode) - $($ep.Url)"
        } catch {
            $allOk = $false
            Write-Err "[错误] $($ep.Name) 响应异常: $($ep.Url)"
        }
    }

    # ---------- 14. 总结 ----------
    Write-Host ""
    Write-Host "  ====================== 启动完成 ======================" -ForegroundColor Green
    Write-Host ""
    Write-Host "  访问地址:" -ForegroundColor Cyan
    if ($StartAll -or $Component -eq "frontend") {
        Write-Host "    前端控制台  http://localhost:$WebPort" -ForegroundColor White
    }
    if ($StartAll -or $Component -eq "backend") {
        Write-Host "    后端 API    http://localhost:$BackendPort" -ForegroundColor White
    }
    if ($StartAll -or $Component -eq "agent") {
        Write-Host "    Agent       http://localhost:$AgentPort" -ForegroundColor White
    }
    Write-Host ""
    Write-Host "  日志目录: $LogDir" -ForegroundColor Gray
    Write-Host "  停止服务: powershell -ExecutionPolicy Bypass -File .\stop.ps1" -ForegroundColor Gray
    Write-Host "  重新部署: powershell -ExecutionPolicy Bypass -File .\restart.ps1" -ForegroundColor Gray
    Write-Host ""
    Write-Host "  数据持久化目录: $BackendDir\data" -ForegroundColor Gray
    Write-Host ""

    if (-not $allOk) {
        Write-Warn "[警告] 部分健康检查未通过，请检查服务日志"
        exit 1
    }
    exit 0
}
catch {
    Write-Host ""
    Write-Err "[失败] 启动流程中断: $_"
    Write-Info "[提示] 可执行 .\stop.ps1 清理已启动的进程后重试"
    exit 1
}
