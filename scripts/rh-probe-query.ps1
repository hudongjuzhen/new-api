$key = "225a26ffc0914646bb66b807e4a2eeff"
$h = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.cn"
$tidStr = "2093183504566235138"   # V2 提交获得
$tidStr2= "2093183501975707649"   # V1 提交获得

Write-Host "==== A. POST task_id as INT64 ===="
foreach ($t in @($tidStr,$tidStr2)) {
  $body = '{"task_id":' + $t + '}'
  Write-Host "--- body=$body ---"
  try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $body
    ($resp | ConvertTo-Json -Depth 20)
  } catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
  }
  Write-Host ""
}

Write-Host "==== B. 旧版 task 查询端点枚举 (V1 风格) ===="
foreach ($t in @($tidStr2)) {
  @(
    @{ name="POST /task/openapi/query /body taskId int";   p="/task/openapi/query";             b='{"taskId":'+$t+'}' },
    @{ name="POST /task/openapi/query /body taskId str";   p="/task/openapi/query";             b='{"taskId":"'+$t+'"}' },
    @{ name="POST /openapi/v1/query task_id";              p="/openapi/v1/query";               b='{"task_id":'+$t+'}' },
    @{ name="POST /openapi/query task_id int";             p="/openapi/query";                  b='{"task_id":'+$t+'}' },
    @{ name="POST /openapi/v2/task/query id int";          p="/openapi/v2/task/query";          b='{"id":'+$t+'}' },
    @{ name="POST /openapi/v2/task/result id int";         p="/openapi/v2/task/result";         b='{"id":'+$t+'}' },
    @{ name="POST /openapi/v2/result task_id int";         p="/openapi/v2/result";              b='{"task_id":'+$t+'}' },
    @{ name="POST /openapi/v2/query both id+task_id int";  p="/openapi/v2/query";               b='{"task_id":'+$t+',"id":'+$t+'}' },
    @{ name="GET  /openapi/v2/query?task_id=...";          p="/openapi/v2/query?task_id=$t";    b=$null },
    @{ name="POST /task/openapi/task/result taskId int";   p="/task/openapi/task/result";       b='{"taskId":'+$t+'}' },
    @{ name="POST /openapi/v2/ai-app/query taskId";        p="/openapi/v2/ai-app/query";        b='{"task_id":'+$t+'}' }
  ) | ForEach-Object {
    Write-Host "--- $($_.name) ---"
    try {
      if ($_.b -eq $null) {
        $resp = Invoke-RestMethod -Method Get -Uri ($d + $_.p) -Headers $h
      } else {
        $resp = Invoke-RestMethod -Method Post -Uri ($d + $_.p) -Headers $h -Body $_.b
      }
      ($resp | ConvertTo-Json -Depth 20)
    } catch {
      if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
    }
    Write-Host ""
  }
}
