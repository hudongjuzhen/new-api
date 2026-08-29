$key = "225a26ffc0914646bb66b807e4a2eeff"
$h = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.cn"

# 重新提交任务 → 等 15s → 多字段 body 试
Write-Host "==== Resubmit ===="
$v2 = '{"instanceType":"default","webhookUrl":"","nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cute cat"}]}'
$resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/run/ai-app/1877265245566922753" -Headers $h -Body $v2
($resp | ConvertTo-Json -Depth 10)
$tid = $resp.taskId
Write-Host "NEW taskId=$tid"

Write-Host "Wait 20s for propagation..."
Start-Sleep -Seconds 20

Write-Host "==== Multi-variant v2/query ===="
@(
  '{"task_id":"'+$tid+'"}',
  '{"task_id":"'+$tid+'","type":"ai-app"}',
  '{"task_id":"'+$tid+'","taskType":"ai-app"}',
  '{"task_id":"'+$tid+'","apiKey":"'+$key+'"}',
  '{"taskId":"'+$tid+'","task_id":"'+$tid+'","apiKey":"'+$key+'"}',
  '{"task_id":"'+$tid+'","resource":"ai-app"}',
  '{"task_id":"'+$tid+'","kind":"ai-app"}',
  '{"task_id":'+$tid+',"apiKey":"'+$key+'"}'
) | ForEach-Object {
  $body = $_
  Write-Host "--- body=$body ---"
  try {
    $r2 = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $body
    ($r2 | ConvertTo-Json -Depth 20)
  } catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
  }
  Write-Host ""
}
