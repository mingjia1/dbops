param(
    [string]$Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

$rootPath = (Resolve-Path $Root).Path
$forbiddenPaths = @(
    '.env',
    'backend\config\config.yaml',
    'agent\encrypt_password.go',
    'agent\decode_password.go',
    'agent\encrypt_only.go',
    'agent\internal\executor\change_password_fix.go'
)

$findings = New-Object System.Collections.Generic.List[string]

foreach ($rel in $forbiddenPaths) {
    $path = Join-Path $rootPath $rel
    if (Test-Path -LiteralPath $path) {
        $findings.Add("Sensitive local file remains in workspace: $rel")
    }
}

$localJsonDumps = Get-ChildItem -LiteralPath (Join-Path $rootPath 'backend') -Filter '*.json' -File -ErrorAction SilentlyContinue
foreach ($file in $localJsonDumps) {
    $findings.Add("Local backend JSON dump remains in workspace: backend\$($file.Name)")
}

$patterns = @(
    '10\.1\.81\.',
    'root:[^@\s]+@tcp\(',
    'DBOPS_(JWT_SECRET|ENCRYPTION_KEY|AGENT_TOKEN)=(?!replace-with|PLEASE-CHANGE|INJECT_VIA_|your-)\S+'
)

$excludeDirs = @('\.git\', '\node_modules\', '\dist\', '\bin\', '\data\', '\logs\', '\.codegraph\')
$files = Get-ChildItem -LiteralPath $rootPath -Recurse -File -Force |
    Where-Object {
        $full = $_.FullName
        -not ($excludeDirs | Where-Object { $full -like "*$_*" }) -and
        $_.Extension -in @('.go', '.ts', '.tsx', '.js', '.json', '.yaml', '.yml', '.env', '.md', '.ps1')
    }

foreach ($file in $files) {
    $rel = $file.FullName.Substring($rootPath.Length).TrimStart('\')
    if ($rel -in @('.env.example', 'backend\config\config.example.yaml', 'scripts\scan-local-secrets.ps1')) {
        continue
    }
    if ($rel -like '*_test.go') {
        continue
    }
    $content = Get-Content -LiteralPath $file.FullName -Raw -ErrorAction SilentlyContinue
    foreach ($pattern in $patterns) {
        if ($content -match $pattern) {
            $findings.Add("Potential secret pattern '$pattern' found in $rel")
        }
    }
}

if ($findings.Count -gt 0) {
    $findings | ForEach-Object { Write-Output $_ }
    exit 1
}

Write-Output "No local secret files or obvious secret patterns found."
