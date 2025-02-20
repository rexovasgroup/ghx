# Set UTF-8 encoding to avoid garbled characters
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

# Define the test package
$testPkg = "./pkg/cmd/attestation/verification"

# Get the current working directory (project root)
$cwd = Get-Location

# Get all directories under pkg/cmd/attestation that contain Go files
$allPkgs = Get-ChildItem -Path "pkg/cmd/attestation" -Recurse -Directory | `
    Where-Object { (Get-ChildItem -Path $_.FullName -Filter "*.go").Count -gt 0 } | `
    ForEach-Object { 
        # Convert absolute path to relative path
        $relativePath = $_.FullName.Replace($cwd, "").TrimStart("\", "/")
        "./$relativePath" -replace "\\", "/"  # Ensure Unix-style paths for Go compatibility
    }

# Remove the test package itself from the list
$allPkgs = $allPkgs | Where-Object { $_ -ne $testPkg }

# Create an array to store failing package pairs
$failingPairs = @()

# Iterate over each package and test it alongside the verification package
foreach ($pkg in $allPkgs) {
    Write-Host "`nTesting $testPkg with $pkg..."

    # Construct the Go test command
    $cmd = " 1..10 | % { go test -tags=integration $testPkg $pkg -count 2 -failfast }"

    # Print the command being run
    Write-Host "Running: $cmd"

    # Execute the command and capture both output and exit code
    $output = Invoke-Expression $cmd 2>&1
    $exitCode = $LASTEXITCODE

    # Print the output of the command
    Write-Host "Output:"
    Write-Host $output

    if ($exitCode -ne 0) {
        Write-Host "Test failed for pair: $testPkg + $pkg"
        $failingPairs += "$testPkg + $pkg"

        # Save the failure output to a log file for further debugging
        $output | Out-File -Append -Encoding utf8 "test_failures.log"
    } else {
        Write-Host "Test passed for pair: $testPkg + $pkg"
    }
}

# Output summary
Write-Host "`n=== Test Run Summary ==="
if ($failingPairs.Count -eq 0) {
    Write-Host "No failing pairs detected."
} else {
    Write-Host "Failing pairs detected:"
    $failingPairs | ForEach-Object { Write-Host $_ }
}

Write-Host "`nCheck 'test_failures.log' for failure details."
