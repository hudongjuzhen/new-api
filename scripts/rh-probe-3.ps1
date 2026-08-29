# 更精准枚举：
# 1) 查 ai-app 列表接口 → 找能跑的简单 app
# 2) 对 2092510860832235521 构造不同 nodeId 尝试（常见 0/1/Input / 字符串 id）
# 3) 测 POST task 查询： /openapi/v2/run/task/{id}/status（run 前缀？）、/openapi/v2/status
$key = "225a26ffc0914646bb66b807e4a2eeff"
$headers = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.ai"

Write-Host "==== [L] 列表接口枚举 ===="
@(
  @{M="Get"; P="/openapi/v2/ai-app/list?page=1&pageSize=5"},
  @{M="Get"; P="/openapi/v2/app/list?page=1&pageSize=5"},
  @{M="Get"; P="/openapi/v2/workflow/list?page=1&pageSize=5"},
  @{M="Post";P="/openapi/v2/ai-app/list"; B=@{page=1; pageSize=5}},
  @{M="Post";P="/openapi/v2/market/list"; B=@{page=1; pageSize=5; type="ai-app"}}
) | ForEach-Object {
    Write-Host "--- $($_.M) $d$($_.P) ---"
    try {
        if ($_.M -eq "Post") {
            $r = Invoke-RestMethod -Method Post -Uri ($d + $_.P) -Headers $headers -Body (ConvertTo-Json $_.B -Depth 8)
        } else {
            $r = Invoke-RestMethod -Method Get -Uri ($d + $_.P) -Headers $headers
        }
        ($r | ConvertTo-Json -Depth 8)
    } catch {
        if ($_.Exception.Response) { $rd = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($rd.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
    }
    Write-Host ""
}

Write-Host "==== [S] 对 Flux2Klein 用不同 node/field 猜法 ===="
$appId = "2092510860832235521"
$attempts = @(
    @(@{nodeId="0"; fieldName="image"; field="image"; fieldValue="https://www.runninghub.cn/favicon.ico"}),
    @(@{nodeId="image_input"; fieldName="image"; field="image"; fieldValue="https://www.runninghub.cn/favicon.ico"}),
    @(@{nodeId="INPUT"; fieldName="image"; field="image"; fieldValue="https://www.runninghub.cn/favicon.ico"}),
    @(@{nodeId=""; fieldName="input_image"; field="image"; fieldValue="https://www.runninghub.cn/favicon.ico"})
)
foreach ($nodes in $attempts) {
    $body = @{ instanceType="default"; usePersonalQueue="false"; nodeInfoList=$nodes } | ConvertTo-Json -Depth 10
    Write-Host "--- nodes=$(($nodes | ConvertTo-Json -Compress)) ---"
    try {
        $r = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/run/ai-app/$appId" -Headers $headers -Body $body
        ($r | ConvertTo-Json -Depth 8)
    } catch {
        if ($_.Exception.Response) { $rd = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($rd.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
    }
    Write-Host ""
}

Write-Host "==== [Q] 任务状态接口更细枚举 ===="
$fake = "000000000000000000000000"
@(
  @{M="Get";  P="/openapi/v2/run/task/$fake/status"},
  @{M="Get";  P="/openapi/v2/task/$fake"},
  @{M="Post"; P="/openapi/v2/task/$fake"; B=@{}},
  @{M="Post"; P="/openapi/v2/run/status"; B=@{taskId=$fake}},
  @{M="Post"; P="/openapi/v2/task/query"; B=@{taskId=$fake}},
  @{M="Post"; P="/openapi/v2/status"; B=@{task_id=$fake; taskId=$fake}}
) | ForEach-Object {
    Write-Host "--- $($_.M) $d$($_.P) ---"
    try {
        if ($_.M -eq "Post") {
            $r = Invoke-RestMethod -Method Post -Uri ($d + $_.P) -Headers $headers -Body (ConvertTo-Json $_.B -Depth 5)
        } else {
            $r = Invoke-RestMethod -Method Get -Uri ($d + $_.P) -Headers $headers
        }
        ($r | ConvertTo-Json -Depth 6)
    } catch {
        if ($_.Exception.Response) { $rd = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($rd.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
    }
    Write-Host ""
}
