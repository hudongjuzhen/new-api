$key = "225a26ffc0914646bb66b807e4a2eeff"
$headers = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$domains = @("https://www.runninghub.cn", "https://www.runninghub.ai")

Write-Host "==== 1. submit speech-2.8-turbo (both domains) ===="
$speechBody = [ordered]@{
  text="Today is a nice day."
  voice="default_female_1"
  emotion="happy"
  speed="1.0"
  volume="1.0"
  pitch="1.0"
} | ConvertTo-Json

$tasks = @()
foreach ($d in $domains) {
    Write-Host "--- POST $d/openapi/v2/rhart-audio/text-to-audio/speech-2.8-turbo ---"
    try {
        $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/rhart-audio/text-to-audio/speech-2.8-turbo" -Headers $headers -Body $speechBody
        $json = ($resp | ConvertTo-Json -Depth 20)
        Write-Host $json
        if ($resp.taskId) { $tasks += @{ Domain=$d; TaskId=$resp.taskId; Status=$resp.status } }
    } catch {
        Write-Host "ERR: $($_.Exception.Message)"
        if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" }
    }
    Write-Host ""
}

Write-Host "==== 2. real /openapi/v2/query for submitted tasks + fake id ===="
$cases = @($tasks)
if ($cases.Count -eq 0) {
    $cases += @{ Domain="https://www.runninghub.cn"; TaskId="12345678901234567890"; Status=$null }
}
foreach ($c in $cases) {
    $d = $c.Domain
    $tid = $c.TaskId
    Write-Host "--- POST $d/openapi/v2/query   taskId=$tid ---"
    try {
        $resp = Invoke-RestMethod -Method Post -Uri "$d/openapi/v2/query" -Headers $headers -Body (ConvertTo-Json @{taskId=$tid})
        ($resp | ConvertTo-Json -Depth 20)
    } catch {
        Write-Host "ERR: $($_.Exception.Message)"
        if ($_.Exception.Response) { $r = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($r.ReadToEnd())" }
    }
    Write-Host ""
}
