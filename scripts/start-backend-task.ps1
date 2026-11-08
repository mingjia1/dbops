# dbops-backend wrapper: load .env then run platform.exe detached.
$repo = "D:\test_tmple\new_dbops"
Get-Content "$repo\.env" | Where-Object { $_ -match '^\s*DBOPS_[A-Za-z_]+\s*=' } | ForEach-Object {
    $k, $v = $_ -split '=', 2
    [Environment]::SetEnvironmentVariable($k.Trim(), $v.Trim(), "Process")
}
Start-Process -FilePath "$repo\backend\bin\platform.exe" `
    -WorkingDirectory $repo -WindowStyle Hidden `
    -RedirectStandardOutput "$repo\logs\backend.log" `
    -RedirectStandardError "$repo\logs\backend.err"
