$key = "225a26ffc0914646bb66b807e4a2eeff"
$headers = @{ Authorization="Bearer $key"; "Content-Type"="application/json" }
$d = "https://www.runninghub.ai"

Write-Host "==== [A] 应用详情（常见枚举）找 nodeInfoList schema ===="
@(
    @{M="Get"; P="/openapi/v2/ai-app/2092510860832235521"},
    @{M="Get"; P="/openapi/v2/workflow/2092510860832235521"},
    @{M="Get"; P="/openapi/v2/ai-app/detail/2092510860832235521"},
    @{M="Get"; P="/openapi/v2/app/detail?id=2092510860832235521"},
    @{M="Post";P="/openapi/v2/ai-app/info"; B=@{appId="2092510860832235521"}},
    @{M="Post";P="/openapi/v2/workflow/detail"; B=@{appId="2092510860832235521"; workflowId="2092510860832235521"}}
) | ForEach-Object {
    Write-Host "--- $($_.M) $d$($_.P) ---"
    try {
        if ($_.M -eq "Post") {
            $r = Invoke-RestMethod -Method Post -Uri ($d + $_.P) -Headers $headers -Body (ConvertTo-Json $_.B -Depth 8)
        } else {
            $r = Invoke-RestMethod -Method Get -Uri ($d + $_.P) -Headers $headers
        }
        ($r | ConvertTo-Json -Depth 16)
    } catch {
        Write-Host "ERR: $($_.Exception.Message)"
        if ($_.Exception.Response) { $reader = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream()); Write-Host "BODY: $($reader.ReadToEnd())" }
    }
    Write-Host ""
}
