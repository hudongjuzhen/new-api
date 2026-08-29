# RH 端点实测脚本
$key = "225a26ffc0914646bb66b807e4a2eeff"

$domains = @(
    "https://www.runninghub.ai",
    "https://www.runninghub.cn"
)

$headers = @{
    "Authorization" = "Bearer $key"
    "Content-Type"  = "application/json"
}

$smallUploadBase64 = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes("fake small test payload"))

Write-Host "==== [1] 提交 ai-app (id=2092510860832235521  Flux2Kleinzimage) on BOTH domains ===="
$appId = "2092510860832235521"
# 找一个不需要必填图片的 ai-app；先试该文档里的 id
$submitBody = @{
    instanceType      = "default"
    usePersonalQueue  = "false"
    nodeInfoList      = @(
        @{
            nodeId    = "1"
            fieldName = "prompt"
            field     = "value"
            fieldValue = "a cat, high quality"
        }
    )
} | ConvertTo-Json -Depth 10

foreach ($d in $domains) {
    Write-Host "--- POST $d/openapi/v2/run/ai-app/$appId ---"
    try {
        $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/run/ai-app/$appId" -Headers $headers -Body $submitBody
        $resp | ConvertTo-Json -Depth 20
    } catch {
        Write-Host "ERR: $($_.Exception.Message)"
        if ($_.Exception.Response) {
            $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream())
            Write-Host "BODY: $($r.ReadToEnd())"
        }
    }
    Write-Host ""
}

Write-Host "==== [2] 任务查询（假设 POST /openapi/v2/task/status 或 /task/{id}/status） 常见 shape 枚举 ===="
# 用一个必然不存在的 task id 看 404 返回体也能推断字段命名风格
$fakeTask = "task_0000000000000000000"
$queries = @(
    @{Method="Post"; Path="/openapi/v2/task/status";       Body=@{task_id=$fakeTask}},
    @{Method="Post"; Path="/openapi/v2/task/$fakeTask/status"; Body=@{task_id=$fakeTask; action="ai-app"}},
    @{Method="Get";  Path="/openapi/v2/task/$fakeTask";       Body=$null},
    @{Method="Post"; Path="/api/v2/task/status";       Body=@{task_id=$fakeTask}},
    @{Method="Post"; Path="/api/v2/task/$fakeTask/status"; Body=@{task_id=$fakeTask}}
)
foreach ($d in $domains | Select-Object -First 1) {
    foreach ($q in $queries) {
        Write-Host "--- $($q.Method) $d$($q.Path) ---"
        try {
            $p = $d + $q.Path
            if ($q.Method -eq "Post") {
                $resp = Invoke-RestMethod -Method Post -Uri $p -Headers $headers -Body ($q.Body | ConvertTo-Json -Depth 10)
            } else {
                $resp = Invoke-RestMethod -Method Get -Uri $p -Headers $headers
            }
            $resp | ConvertTo-Json -Depth 20
        } catch {
            Write-Host "ERR: $($_.Exception.Message)"
            if ($_.Exception.Response) {
                $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream())
                Write-Host "BODY: $($r.ReadToEnd())"
            }
        }
        Write-Host ""
    }
}

Write-Host "==== [3] 用户信息 / 余额接口（连通性测试用） 常见枚举 ===="
$infos = @(
    @{Method="Get";  Path="/openapi/v2/user/info"},
    @{Method="Get";  Path="/openapi/v2/user/wallet"},
    @{Method="Get";  Path="/api/v2/user/info"},
    @{Method="Get";  Path="/openapi/v2/account/info"}
)
foreach ($d in $domains) {
    foreach ($q in $infos) {
        Write-Host "--- $($q.Method) $d$($q.Path) ---"
        try {
            $resp = Invoke-RestMethod -Method $q.Method -Uri ($d + $q.Path) -Headers $headers
            $resp | ConvertTo-Json -Depth 20
        } catch {
            Write-Host "ERR: $($_.Exception.Message)"
            if ($_.Exception.Response) {
                $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream())
                Write-Host "BODY: $($r.ReadToEnd())"
            }
        }
    }
    Write-Host ""
}
