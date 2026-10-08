# 世界引擎（本机 Windows 联调用）—— 把 new-api 按「引擎已经部署好」的样子起起来。
#
# 为什么需要这个脚本，而不是往 `.env` 里加几行：
#
#   `zsy/world` 的配置（`config.go` 里的 `var cfg = Config{...}`）是**包级变量**，
#   在 Go 的包初始化阶段就求值一次 —— 而那发生在 `main()` 之前，
#   更发生在 `main.go: InitResources()` 里那句 `godotenv.Load(".env")` **之前**
#   （`zsy/extbootstrap` 是 `main.go` 第 35 行的空导入，包初始化先于函数体）。
#   所以 `ZSY_WORLD_ENGINE` 写进 `.env` **不会生效**，必须进真正的进程环境。
#
#   实测证据：本机 3000 那个进程在没设环境变量时，`project.create` 回
#   `E_UPSTREAM 世界引擎不可用：找不到或无法执行 world-parser-svc（…）`。
#
# ⚠ 这里指向的是 **release-check 那份 Windows 构建**，只为本机联调。
#   它已确认**没有** `--features test-provider` 那扇"白拿结果"的后门
#   （在二进制里搜 test-provider / TestProvider 无命中）。
#   正式服要换成 Linux 上 `cargo build --release -p world-parser --bin world-parser-svc` 的产物。
#
# ⚠ 这个脚本只负责**把引擎接上**。它不授予任何能力 —— 「签发 ≠ 授予」那条规矩
#   对引擎同样适用：接上引擎只是让 `project.create` 这一步有东西可调，
#   账号没有 `world-ip` 照样 403（见 docs/24 §4）。

[CmdletBinding()]
param(
    # new-api 从哪个目录起（它的 .env / plugin-templates 都按当前目录解析）
    [string]$Repo = 'D:\work\new-api',
    # 端口。客户端那一屏认的是 http://localhost:3000（config.rs 的 DEFAULT_ACCOUNT_BASE）
    [int]$Port = 3000,
    # 引擎仓库根目录
    [string]$EngineRepo = 'D:\work\Matrix Tech World Parsing Format',
    # 引擎可执行文件（默认取 release-check 那份 Windows 构建）
    [string]$EnginePath = '',
    # 校验器 shim
    [string]$ValidatorSvc = '',
    # 只做那两个 sidecar 自检，不起服务。
    # 它的用处是：端口被占着（本机已经有一次启动）时仍然能回答"引擎接上以后到底通不通"。
    [switch]$CheckOnly,
    # 传给 new-api 的其它参数（默认与本机既有的那次启动一致）
    # ⚠ 它必须是最后一个参数：PowerShell 里带默认值的数组参数后面不能再跟 `[switch]`
    #   （`@('a','b'),` 之后接 `[switch]` 会被解析成"数组里又多一项"，报
    #   `Missing expression after ','`）。
    [string[]]$ExtraArgs = @('--log-dir', '.\logs')
)

$ErrorActionPreference = 'Stop'

if (-not $EnginePath) {
    $EnginePath = Join-Path $EngineRepo 'target\release-check\release\world-parser-svc.exe'
}
if (-not $ValidatorSvc) {
    $ValidatorSvc = Join-Path $EngineRepo 'schema\1.1\world-validator-svc.mjs'
}

# ── 1. 先把两个 sidecar 各跑一次，确认它们真的能起来 ────────────────────────
# 顺序就是 `op_project.go: opProjectCreate` 的顺序：先引擎产空文档，再闸门校验。
# 只检查"文件在不在"是不够的：路径对、二进制却能缺 DLL / 缺 Node 模块，
# 那会在一次真实请求里表现成 E_UPSTREAM，而不是在这里。

foreach ($p in @($EnginePath, $ValidatorSvc)) {
    if (-not (Test-Path $p)) { throw "缺少 $p" }
}

$node = (Get-Command node -ErrorAction SilentlyContinue).Source
if (-not $node) { throw 'PATH 里没有 node —— 校验器 shim 需要它' }

Write-Host '[1/3] 引擎自检：doc.new'
$docReply = '{"op":"doc.new","novel_title":"自检"}' | & $EnginePath
$doc = ($docReply | ConvertFrom-Json)
if ($doc.status -ne 'ok') { throw "引擎自检失败：$docReply" }
Write-Host ("      空文档 OK（format={0} version={1}）" -f $doc.doc.format, $doc.doc.version)

Write-Host '[2/3] 校验器自检：拿引擎刚产的那份空文档过权威校验'
# ⚠ 必须用**引擎真产出的**文档，不能用这里手写的夹具 —— 那正是 §12.7 偏差 10
#   翻车的方式（手写夹具与引擎对"合法 MTW"的理解不一致，只有跑校验器才发现）。
$req = @{ op = 'validate'; doc = $doc.doc } | ConvertTo-Json -Depth 20 -Compress
$verdict = ($req | & $node $ValidatorSvc) | ConvertFrom-Json
if ($verdict.status -ne 'ok') { throw "校验器自检失败：$($verdict.error.message)" }
$inner = $verdict.report.report[0]
if (-not $inner.shapeOk) {
    # ★ 这不是脚本坏了，而是**它本来要抓的那一类问题**：
    #   `doc.new` 的产物过不了 schema 时，`project.create` 会回 E_INPUT，
    #   客户端那一屏就会说"参数产出的空文档未通过权威校验"。
    Write-Warning ("校验器拒绝了引擎的空文档：{0}" -f ($inner.shapeErrors -join '; '))
    Write-Warning '  ⇒ project.create 现在会回 E_INPUT（这是真实缺陷，不是环境问题）'
} else {
    Write-Host '      空文档通过权威校验 OK'
}

# ── 2. 起服务 ──────────────────────────────────────────────────────────────
if ($CheckOnly) {
    Write-Host ''
    Write-Host '（-CheckOnly：自检到此为止，没有起服务）'
    return
}

Write-Host '[3/3] 起 new-api（带引擎环境变量）'
$env:ZSY_WORLD_ENGINE = $EnginePath
$env:ZSY_WORLD_VALIDATOR_SVC = $ValidatorSvc
$env:ZSY_WORLD_NODE = $node
# 抽取一次可能要十几分钟；默认 30 秒会让 ingest.run 必然超时
$env:ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS = '900'

Write-Host "      ZSY_WORLD_ENGINE          = $EnginePath"
Write-Host "      ZSY_WORLD_VALIDATOR_SVC   = $ValidatorSvc"
Write-Host "      ZSY_WORLD_NODE            = $node"
Write-Host "      ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS = 900"
Write-Host ''
Write-Host "      .\new-api.exe --port $Port $($ExtraArgs -join ' ')"
Write-Host ''

Set-Location $Repo
& (Join-Path $Repo 'new-api.exe') --port $Port @ExtraArgs
