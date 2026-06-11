param(
    [string]$Scenario = ".\examples\scenarios\checkout-idempotency-race.json"
)

Write-Host "Start the demo API in one terminal:"
Write-Host "  go run ./examples/demo-api/cmd"
Write-Host ""
Write-Host "Start the Wreckr API in another terminal:"
Write-Host "  go run ./apps/api/cmd/api"
Write-Host ""
Write-Host "Run a scenario:"
Write-Host "  go run ./apps/api/cmd/wreckr run $Scenario"
