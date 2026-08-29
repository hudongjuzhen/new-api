$key = "225a26ffc0914646bb66b807e4a2eeff"
$h = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.cn"

Write-Host "==== V1 旧协议 /task/openapi/ai-app/run ===="
$v1 = '{"webappId":1877265245566922753,"apiKey":"' + $key + '","instanceType":"default","nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cute cat sitting in classroom"}]}'
try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/task/openapi/ai-app/run" -Headers $h -Body $v1
    ($resp | ConvertTo-Json -Depth 20)
} catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
}
Write-Host ""

Write-Host "==== V2 新协议 /openapi/v2/run/ai-app/1877265245566922753 (同 id) ===="
$v2 = '{"instanceType":"default","webhookUrl":"","nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cute cat sitting in classroom"}]}'
try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/run/ai-app/1877265245566922753" -Headers $h -Body $v2
    ($resp | ConvertTo-Json -Depth 20)
    $tid = ""
    if ($resp.taskId) { $tid = $resp.taskId }
    if ($resp.data -and $resp.data.taskId) { $tid = $resp.data.taskId.ToString() }
    if ($tid -ne "") {
        Write-Host ">>> got taskId=$tid"
        for ($i=0; $i -lt 6; $i++) {
            Start-Sleep -Seconds 8
            $qbody = '{"task_id":"' + $tid + '"}'
            Write-Host "--- poll $i body=$qbody ---"
            try {
                $r2 = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $qbody
                ($r2 | ConvertTo-Json -Depth 20)
                if ($r2.status -eq "SUCCESS" -or $r2.status -eq "FAILED") { break }
            } catch {
                if ($_.Exception.Response) { $rd = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($rd.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
            }
        }
    }
} catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
}
