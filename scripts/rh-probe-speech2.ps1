$key = "225a26ffc0914646bb66b807e4a2eeff"
$h = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.cn"

Write-Host "==== v2/query 多种 body 形状 ===="
@(
  @{ name="task_id_snakecase"; body='{"task_id":"12345678901234567890"}' },
  @{ name="task_id_camelcase"; body='{"taskId":"12345678901234567890"}' },
  @{ name="Id_uppercase_snake";  body='{"TaskID":"12345678901234567890"}' },
  @{ name="both"; body='{"taskId":"12345678901234567890","task_id":"12345678901234567890"}' },
  @{ name="array"; body='{"taskIds":["12345678901234567890"]}' }
) | ForEach-Object {
  Write-Host "--- $($_.name) body=$($_.body) ---"
  try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $_.body
    ($resp | ConvertTo-Json -Depth 20)
  } catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
  }
  Write-Host ""
}

Write-Host "==== speech voice_id correct 尝试 ======"
# 再次校验 body 编码无误
$body = '{"text":"Today is a nice day.","voice_id":"default_female_1","emotion":"happy","speed":1.0,"volume":1.0,"pitch":1.0}'
try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/rhart-audio/text-to-audio/speech-2.8-turbo" -Headers $h -Body $body
    ($resp | ConvertTo-Json -Depth 20)
    if ($resp.taskId) {
        Start-Sleep -Seconds 3
        $qbody = '{"taskId":"' + $resp.taskId + '"}'
        Write-Host "--- query with body: $qbody ---"
        $r2 = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $qbody
        ($r2 | ConvertTo-Json -Depth 20)
    }
} catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
}
