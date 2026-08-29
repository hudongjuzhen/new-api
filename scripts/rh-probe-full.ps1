$key = "225a26ffc0914646bb66b807e4a2eeff"
$h = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.cn"

Write-Host "==== submit speech 2.8 turbo ===="
$body = '{"text":"Today is a nice day and we should go outside.","voice_id":"default_female_1","emotion":"happy","speed":1.0,"volume":1.0,"pitch":1.0,"enable_base64_output":false,"response_format":"mp3","english_normalization":true}'
try {
    $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/rhart-audio/text-to-audio/speech-2.8-turbo" -Headers $h -Body $body
    ($resp | ConvertTo-Json -Depth 20)
    if ($resp.taskId) {
        $tid = $resp.taskId
        Write-Host "Got taskId: $tid  -- polling 5s 6 times"
        for ($i = 0; $i -lt 6; $i++) {
            Start-Sleep -Seconds 5
            $qbody = '{"task_id":"' + $tid + '"}'
            Write-Host "--- poll $i POST $d/openapi/v2/query body=$qbody ---"
            try {
                $r2 = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $h -Body $qbody
                ($r2 | ConvertTo-Json -Depth 20)
                if ($r2.status -eq "SUCCESS" -or $r2.status -eq "FAILED") { break }
            } catch {
                if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
            }
        }
    }
} catch {
    if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" } else { Write-Host "ERR: $($_.Exception.Message)" }
}
