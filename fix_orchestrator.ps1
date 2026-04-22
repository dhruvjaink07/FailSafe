$content = Get-Content internal\orchestrator\orchestrator.go
$newContent = @()
foreach ($line in $content) {
    if ($line -match "import \(") {
        $newContent += $line
        $newContent += "`"net/http`""
    } else {
        $newContent += $line
    }
}
$newContent | Set-Content internal\orchestrator\orchestrator.go
