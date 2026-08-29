$key = "225a26ffc0914646bb66b807e4a2eeff"
$h = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.cn"
$tid = "2093184131443990530"  # 刚提交的 V2 任务

$bodies = @(
  '{"taskId":"'+$tid+'"}',
  '{"taskId":"'+$tid+'","apiKey":"'+$key+'"}',
  '{"task_id":"'+$tid+'","apiKey":"'+$key+'"}',
  '{"task_id":"'+$tid+'","apikey":"'+$key+'"}',
  '{"taskId":"'+$tid+'","Authorization":"Bearer '+$key+'"}',
  '{"taskId":"'+$tid+'","platform":"webapp"}',
  '{"taskId":"'+$tid+'","workflowId":"1876205853438365698"}'
)
foreach ($b in $bodies) {
  Write-Host "--- POST /openapi/v2/query body=$b ---"
  try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $b
    ($resp | ConvertTo-Json -Depth 20)
  } catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
  }
  Write-Host ""
}
