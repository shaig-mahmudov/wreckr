param(
    [ValidateSet("up", "down", "force", "version")]
    [string]$Command = "up",
    [string]$Steps = ""
)

$dockerArgs = @(
    "compose",
    "--profile",
    "tools",
    "run",
    "--rm",
    "migrate"
)

if ($Command -ne "up" -or $Steps -ne "") {
    $dockerArgs += @(
        "-path=/migrations",
        "-database=postgres://wreckr:wreckr@postgres:5432/wreckr?sslmode=disable",
        $Command
    )
}

if ($Steps -ne "") {
    $dockerArgs += $Steps
}

docker @dockerArgs
