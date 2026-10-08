# 本机联调：按"引擎已经部署好"的样子把 new-api 起起来（前台阻塞，交给计划任务/终端即可）。
#
# ⚠ 为什么必须有一个"设环境变量再起"的壳，而不是直接用 exe：
#
#   `zsy/world` 的配置（`config.go` 里的 `var cfg = Config{...}`）是**包级变量**，
#   在 Go 的包初始化阶段求值一次 —— 那发生在 `main()` 之前，更发生在
#   `main.go: InitResources()` 里那句 `godotenv.Load(".env")` **之前**。
#   所以 `ZSY_WORLD_*` 写进 `.env` **不生效**，它必须进**进程环境**。
#   （同目录的 `world-deploy-local.ps1` 顶部有完整实测记录。）
#
# ⚠ 它和被它替下来的那条启动方式的区别：
#
#   2026-10-08 之前是 `new-api-local.exe --log-dir .\logs` 直接在某个控制台里跑，
#   `ZSY_WORLD_*` 靠那个控制台会话的环境变量。于是"换个地方起"就必然踩到
#   "世界引擎不可用：找不到或无法执行 world-parser-svc"——环境是隐式的。
#   这个脚本把它显式写下来，任何人都能用同一条命令复现。
#
# 用法（前台跑；要后台就配计划任务或 `Start-Process`）：
#
#   pwsh -NoProfile -File D:\work\new-api\_scripts\world-serve-local.ps1

$ErrorActionPreference = 'Stop'

$Repo = 'D:\work\new-api'
$EngineRepo = 'D:\work\Matrix Tech World Parsing Format'
$Exe = Join-Path $Repo 'new-api-local.exe'
$Engine = Join-Path $EngineRepo 'target\release-check\release\world-parser-svc.exe'
$Validator = Join-Path $EngineRepo 'schema\1.1\world-validator-svc.mjs'

foreach ($p in @($Exe, $Engine, $Validator)) {
    if (-not (Test-Path $p)) { throw "缺少 $p" }
}

$node = (Get-Command node -ErrorAction SilentlyContinue).Source
if (-not $node) { $node = 'C:\nvm4w\nodejs\node.exe' }
if (-not (Test-Path $node)) { throw "找不到 node（校验器 shim 需要它）：$node" }

$env:ZSY_WORLD_ENGINE = $Engine
$env:ZSY_WORLD_VALIDATOR_SVC = $Validator
$env:ZSY_WORLD_NODE = $node
# ★ 一次整本解析实测要 41 分钟（39 次模型调用）。默认 30 秒会让 ingest.run
#   必然超时；900（部署脚本用的值）对整本也不够，这里给 3600。
$env:ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS = '3600'

Set-Location $Repo
Write-Host "ZSY_WORLD_ENGINE                  = $Engine"
Write-Host "ZSY_WORLD_VALIDATOR_SVC           = $Validator"
Write-Host "ZSY_WORLD_NODE                    = $node"
Write-Host "ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS = $($env:ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS)"
Write-Host "起：$Exe --log-dir .\logs（端口默认 3000）"
Write-Host ''

& $Exe --log-dir .\logs
