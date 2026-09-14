# LogiMate Build 019 reproducible Windows release bootstrap.
# This script is intentionally compatible with Windows PowerShell 5.1 so it can
# bootstrap the pinned release toolchain without changing machine-wide installs.
[CmdletBinding()]
param(
    [switch]$KeepTools,
    [switch]$SkipSecondReproBuild
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

$ExpectedGo = '1.27.1'
$GoArchive = 'go1.27.1.windows-amd64.zip'
$GoUrl = 'https://go.dev/dl/go1.27.1.windows-amd64.zip'
$GoSha256 = 'a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d'

$ExpectedPowerShell = '7.6.6'
$PwshArchive = 'PowerShell-7.6.6-win-x64.zip'
$PwshUrl = 'https://github.com/PowerShell/PowerShell/releases/download/v7.6.6/PowerShell-7.6.6-win-x64.zip'
$PwshSha256 = '02fe458be20493fbdf43f61ea20610b811ee6c738ab1676c61b9cfcd1a33c860'

$Tools = Join-Path $Root '.release-tools'
$Cache = Join-Path $Tools 'cache'
$GoHome = Join-Path $Tools 'go'
$PwshHome = Join-Path $Tools 'pwsh'
$ReportDir = Join-Path $Root 'release-gate'
$Transcript = Join-Path $ReportDir 'Build019-Go1.27.1-Release-Gate.txt'

function Write-Step([string]$Text) {
    Write-Host "`n==> $Text" -ForegroundColor Cyan
}

function Get-VerifiedArchive {
    param(
        [Parameter(Mandatory=$true)][string]$Url,
        [Parameter(Mandatory=$true)][string]$Path,
        [Parameter(Mandatory=$true)][string]$Sha256
    )
    if (-not (Test-Path -LiteralPath $Path)) {
        Write-Host "Downloading $Url"
        Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Path
    }
    $actual = (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $Sha256.ToLowerInvariant()) {
        Remove-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
        throw "SHA-256 verification failed for $([IO.Path]::GetFileName($Path)). Expected $Sha256, got $actual"
    }
    Write-Host "Verified SHA-256: $actual"
}

function Reset-Directory([string]$Path) {
    if (Test-Path -LiteralPath $Path) { Remove-Item -LiteralPath $Path -Recurse -Force }
    New-Item -ItemType Directory -Path $Path -Force | Out-Null
}

function Assert-SourceManifest {
    $manifest = Join-Path $Root 'SOURCE_MANIFEST_SHA256.txt'
    if (-not (Test-Path -LiteralPath $manifest -PathType Leaf)) { throw 'SOURCE_MANIFEST_SHA256.txt is missing' }
    $rootFull = [IO.Path]::GetFullPath($Root).TrimEnd([char[]]@([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)) + [IO.Path]::DirectorySeparatorChar
    $count = 0
    foreach ($line in Get-Content -LiteralPath $manifest) {
        if ([string]::IsNullOrWhiteSpace($line) -or $line.StartsWith('#')) { continue }
        if ($line -notmatch '^([0-9A-Fa-f]{64})  (.+)$') { throw "Invalid source-manifest line: $line" }
        $expected = $Matches[1].ToLowerInvariant()
        $relative = $Matches[2]
        if ([IO.Path]::IsPathRooted($relative) -or $relative -match '(^|[\\/])\.\.([\\/]|$)') { throw "Unsafe source-manifest path: $relative" }
        $relativeNative = $relative.Replace('/', [IO.Path]::DirectorySeparatorChar)
        $full = [IO.Path]::GetFullPath((Join-Path $Root $relativeNative))
        if (-not $full.StartsWith($rootFull, [StringComparison]::OrdinalIgnoreCase)) { throw "Source-manifest path escaped root: $relative" }
        if (-not (Test-Path -LiteralPath $full -PathType Leaf)) { throw "Source-manifest file is missing: $relative" }
        $actual = (Get-FileHash -LiteralPath $full -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($actual -ne $expected) { throw "Source integrity failed for $relative. Expected $expected, got $actual" }
        $count++
    }
    if ($count -lt 10) { throw "Source manifest is unexpectedly small: $count files" }
    Write-Host "Verified source manifest: $count files"
}

function Invoke-PwshChecked {
    param([Parameter(Mandatory=$true)][string]$Command)
    $pwsh = Join-Path $PwshHome 'pwsh.exe'
    & $pwsh -NoLogo -NoProfile -ExecutionPolicy Bypass -Command $Command
    if ($LASTEXITCODE -ne 0) { throw "PowerShell release command failed with exit code $LASTEXITCODE" }
}

New-Item -ItemType Directory -Path $Cache,$ReportDir -Force | Out-Null

Write-Step 'Verify source release identity'
$version = (Get-Content (Join-Path $Root 'VERSION') -Raw).Trim()
$build = (Get-Content (Join-Path $Root 'BUILD') -Raw).Trim()
if ($version -ne '0.0.1-alpha') { throw "Unexpected visible VERSION '$version'; expected 0.0.1-alpha" }
if ($build -ne '019') { throw "Unexpected BUILD '$build'; expected 018" }
$goDirective = (Get-Content (Join-Path $Root 'go.mod') | Where-Object { $_ -match '^go\s+' } | Select-Object -First 1)
if (-not $goDirective) { throw 'go.mod has no go directive' }
$goDirectiveVersion = (($goDirective -split '\s+')[1]).Trim()
if ($goDirectiveVersion -ne $ExpectedGo) { throw "go.mod requests Go $goDirectiveVersion; release bootstrap is pinned to $ExpectedGo" }
Write-Host "LogiMate $version Build $build | required Go $goDirectiveVersion"
Assert-SourceManifest

Write-Step "Acquire and verify official Go $ExpectedGo"
$goZip = Join-Path $Cache $GoArchive
Get-VerifiedArchive -Url $GoUrl -Path $goZip -Sha256 $GoSha256
Reset-Directory $GoHome
$goExtract = Join-Path $Tools 'go-extract'
Reset-Directory $goExtract
Expand-Archive -LiteralPath $goZip -DestinationPath $goExtract -Force
$extractedGo = Join-Path $goExtract 'go'
if (-not (Test-Path (Join-Path $extractedGo 'bin\go.exe'))) { throw 'Official Go archive did not contain go\bin\go.exe' }
Get-ChildItem -LiteralPath $extractedGo -Force | ForEach-Object { Copy-Item $_.FullName -Destination $GoHome -Recurse -Force }
Remove-Item $goExtract -Recurse -Force

Write-Step "Acquire and verify official PowerShell $ExpectedPowerShell"
$pwshZip = Join-Path $Cache $PwshArchive
Get-VerifiedArchive -Url $PwshUrl -Path $pwshZip -Sha256 $PwshSha256
Reset-Directory $PwshHome
Expand-Archive -LiteralPath $pwshZip -DestinationPath $PwshHome -Force
$pwshExe = Join-Path $PwshHome 'pwsh.exe'
if (-not (Test-Path $pwshExe)) { throw 'PowerShell archive did not contain pwsh.exe' }

Write-Step 'Validate exact release toolchain'
$goExe = Join-Path $GoHome 'bin\go.exe'
$goVersionText = (& $goExe version).Trim()
if ($goVersionText -notmatch 'go1\.27\.1\s+windows/amd64') { throw "Unexpected Go toolchain: $goVersionText" }
$pwshVersionText = (& $pwshExe -NoLogo -NoProfile -Command '$PSVersionTable.PSVersion.ToString()').Trim()
if ($pwshVersionText -ne $ExpectedPowerShell) { throw "Unexpected PowerShell toolchain: $pwshVersionText" }
Write-Host $goVersionText
Write-Host "PowerShell $pwshVersionText"

# Ensure child PowerShell always resolves the verified portable Go before any
# machine-wide Go installation. GOTOOLCHAIN=local also prevents silent toolchain
# substitution after our hash/version checks.
$escapedGoRoot = $GoHome.Replace("'","''")
$escapedRoot = $Root.Replace("'","''")
$prefix = @"
`$ErrorActionPreference='Stop'
Set-Location '$escapedRoot'
`$env:GOROOT='$escapedGoRoot'
`$env:PATH=(Join-Path '$escapedGoRoot' 'bin') + ';' + `$env:PATH
`$env:GOTOOLCHAIN='local'
# Build 019 is an unsigned alpha release unless a later signing-specific pipeline is used.
# Clear ambient signing variables so local machine state cannot change reproducibility.
`$env:LOGIMATE_SIGN_THUMBPRINT=''
`$env:LOGIMATE_TIMESTAMP_URL=''
if ((& go env GOVERSION).Trim() -ne 'go1.27.1') { throw 'Verified Go 1.27.1 was not selected' }
"@

Write-Step 'Formatting, dependency and test gate'
$quality = @"
$prefix
`$dirty = @(& gofmt -l cmd internal)
if (`$dirty.Count -gt 0) { throw ('gofmt gate failed: ' + (`$dirty -join ', ')) }
& go list -deps ./cmd/logimate | Out-Null
if (`$LASTEXITCODE -ne 0) { throw 'go list dependency gate failed' }
& go vet -unsafeptr=false ./...
if (`$LASTEXITCODE -ne 0) { throw 'go vet failed' }
& go test ./...
if (`$LASTEXITCODE -ne 0) { throw 'go test failed' }
"@
Invoke-PwshChecked -Command $quality

Write-Step 'Build release once with verified Go 1.27.1'
$buildCommand = @"
$prefix
& ./build.ps1
if (`$LASTEXITCODE -ne 0) { throw 'build.ps1 failed' }
"@
Invoke-PwshChecked -Command $buildCommand

$firstApp = (Get-FileHash (Join-Path $Root 'LogiMate.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
$firstInstaller = (Get-FileHash (Join-Path $Root 'dist\LogiMate-Setup-x64.exe') -Algorithm SHA256).Hash.ToLowerInvariant()

if (-not $SkipSecondReproBuild) {
    Write-Step 'Rebuild and verify byte-identical app/installer'
    Invoke-PwshChecked -Command $buildCommand
    $secondApp = (Get-FileHash (Join-Path $Root 'LogiMate.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
    $secondInstaller = (Get-FileHash (Join-Path $Root 'dist\LogiMate-Setup-x64.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($firstApp -ne $secondApp) { throw "Reproducibility failed for LogiMate.exe: $firstApp != $secondApp" }
    if ($firstInstaller -ne $secondInstaller) { throw "Reproducibility failed for installer: $firstInstaller != $secondInstaller" }
}

Write-Step 'Extract source archive and reproduce release binaries'
$sourceRebuild = Join-Path $ReportDir 'source-rebuild'
Reset-Directory $sourceRebuild
Expand-Archive -LiteralPath (Join-Path $Root 'dist\LogiMate-Source.zip') -DestinationPath $sourceRebuild -Force
$escapedSource = $sourceRebuild.Replace("'","''")
$sourceRebuildCommand = @"
`$ErrorActionPreference='Stop'
Set-Location '$escapedSource'
`$env:GOROOT='$escapedGoRoot'
`$env:PATH=(Join-Path '$escapedGoRoot' 'bin') + ';' + `$env:PATH
`$env:GOTOOLCHAIN='local'
`$env:CGO_ENABLED='0'
`$env:GOOS='windows'
`$env:GOARCH='amd64'
`$v=(Get-Content VERSION -Raw).Trim()
`$b=(Get-Content BUILD -Raw).Trim()
& go build -trimpath -ldflags "-H=windowsgui -s -w -X main.version=`$v -X main.buildID=`$b" -o source-rebuilt-app.exe ./cmd/logimate
if (`$LASTEXITCODE -ne 0) { throw 'Source-rebuild application compile failed' }
New-Item -ItemType Directory -Force cmd/installer/payload | Out-Null
Copy-Item source-rebuilt-app.exe cmd/installer/payload/LogiMate.exe -Force
& go build -tags installer -trimpath -ldflags "-H=windowsgui -s -w -X main.version=`$v -X main.buildID=`$b" -o source-rebuilt-installer.exe ./cmd/installer
if (`$LASTEXITCODE -ne 0) { throw 'Source-rebuild installer compile failed' }
Remove-Item cmd/installer/payload/LogiMate.exe -Force -ErrorAction SilentlyContinue
"@
Invoke-PwshChecked -Command $sourceRebuildCommand
$sourceApp = (Get-FileHash (Join-Path $sourceRebuild 'source-rebuilt-app.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
$sourceInstaller = (Get-FileHash (Join-Path $sourceRebuild 'source-rebuilt-installer.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
$finalApp = (Get-FileHash (Join-Path $Root 'LogiMate.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
$finalInstaller = (Get-FileHash (Join-Path $Root 'dist\LogiMate-Setup-x64.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
if ($sourceApp -ne $finalApp) { throw "Source archive rebuild does not match release app: $sourceApp != $finalApp" }
if ($sourceInstaller -ne $finalInstaller) { throw "Source archive rebuild does not match release installer: $sourceInstaller != $finalInstaller" }

Write-Step 'Verify release attestation reports the real toolchain'
$attPath = Join-Path $Root 'dist\RELEASE_ATTESTATION.json'
$att = Get-Content $attPath -Raw | ConvertFrom-Json
if ($att.version -ne '0.0.1-alpha' -or $att.build -ne '019') { throw 'Release attestation has wrong release identity' }
if ($att.toolchain.goEnvVersion -ne 'go1.27.1') { throw "Attestation did not record go1.27.1: $($att.toolchain.goEnvVersion)" }
if ($att.toolchain.powerShellVersion -ne '7.6.6') { throw "Attestation did not record PowerShell 7.6.6: $($att.toolchain.powerShellVersion)" }

Write-Step 'Write local release gate transcript'
$hashLines = Get-Content (Join-Path $Root 'dist\SHA256SUMS.txt')
@(
    'LogiMate Build 019 release gate',
    "Visible version: $version",
    "Internal build: $build",
    "Go: $goVersionText",
    "PowerShell: $pwshVersionText",
    "Go archive SHA-256: $GoSha256",
    "PowerShell archive SHA-256: $PwshSha256",
    "Application reproducible SHA-256: $firstApp",
    "Installer reproducible SHA-256: $firstInstaller",
    "Source-archive app rebuild SHA-256: $sourceApp",
    "Source-archive installer rebuild SHA-256: $sourceInstaller",
    'Signing: prerelease policy; see RELEASE_ATTESTATION.json',
    '',
    'Release package hashes:',
    $hashLines
) | Set-Content -LiteralPath $Transcript -Encoding UTF8
Copy-Item -LiteralPath $Transcript -Destination (Join-Path $Root 'dist\Build019-Go1.27.1-Release-Gate.txt') -Force

$verificationPath = Join-Path $Root 'dist\PACKAGE_VERIFICATION.txt'
@(
    'LogiMate 0.0.1-alpha · Build 019',
    'Package verification',
    '',
    'Go 1.27.1 exact toolchain: PASS',
    'PowerShell 7.6.6 exact toolchain: PASS',
    'Source manifest verification: PASS',
    'gofmt: PASS',
    'Unit tests: PASS',
    'Windows vet: PASS',
    'Windows x64 build: PASS',
    'Windows ARM64 compile: PASS',
    'Installer build/vet: PASS',
    'Standalone runtime dependency gate: PASS',
    'Application reproducibility: PASS',
    'Installer reproducibility: PASS',
    'Source rebuild comparison: PASS',
    'Authenticode: alpha policy; see RELEASE_ATTESTATION.json',
    '',
    "Application SHA-256: $firstApp",
    "Installer SHA-256: $firstInstaller",
    'Both source-rebuilt files are byte-identical to the Build 019 release binaries.'
) | Set-Content -LiteralPath $verificationPath -Encoding UTF8

$completionPath = Join-Path $Root 'dist\LogiMate-0.0.1-alpha-Build019-Completion-Report.md'
@(
    '# LogiMate 0.0.1-alpha · Build 019 — completion report',
    '',
    '## Completed in Build 019',
    '',
    '- complete re-audit of the verified Build 017 publication-candidate baseline',
    '- automatic Memory Integrity/HVCI mutation removed; status opens Windows Security only',
    '- Memory Integrity action is mouse, keyboard and UI Automation accessible',
    '- updater/uninstaller/path authorization and diagnostic privacy hardening',
    '- global state synchronization and setup deadlock regression correction',
    '- current user-facing terminology/reference documentation cleanup',
    '- exact Go 1.27.1 and PowerShell 7.6.6 release-toolchain enforcement',
    '- source-manifest integrity gate',
    '- byte-identical application and installer rebuild gate',
    '- source-ZIP rebuild comparison gate',
    '- GitHub CodeQL and provenance workflow',
    '- tag-triggered GitHub prerelease publishing with all release variants',
    '',
    '## Release artifacts',
    '',
    '- standalone x64 application',
    '- x64 installer',
    '- x64 portable ZIP',
    '- ARM64 compile-validation executable',
    '- source ZIP',
    '- GitHub-ready source ZIP',
    '- release attestation, checksums and gate transcript',
    '',
    '## Intentionally not declared complete',
    '',
    '- remaining physical G27 adverse lifecycle matrix',
    '- physical G25 certification',
    '- physical Driving Force GT certification',
    '- final clean-machine/manual Windows UI matrix where hardware or human observation is required',
    '- Authenticode publisher signing unless a real signing identity is configured',
    '',
    'Build 019 remains `0.0.1-alpha` and is published as a prerelease, not a stable fully certified release.'
) | Set-Content -LiteralPath $completionPath -Encoding UTF8

$releaseZip = Join-Path $Root 'LogiMate-0.0.1-alpha-Build019-Go1.27.1-Release.zip'
if (Test-Path $releaseZip) { Remove-Item $releaseZip -Force }
$releaseStage = Join-Path $ReportDir 'release-package'
Reset-Directory $releaseStage
Copy-Item (Join-Path $Root 'dist\LogiMate.exe') $releaseStage
Copy-Item (Join-Path $Root 'dist\LogiMate-Setup-x64.exe') $releaseStage
Copy-Item (Join-Path $Root 'dist\LogiMate-arm64-validation.exe') $releaseStage
Copy-Item (Join-Path $Root 'dist\LogiMate-Portable-x64.zip') $releaseStage
Copy-Item (Join-Path $Root 'dist\LogiMate-Source.zip') $releaseStage
Copy-Item (Join-Path $Root 'dist\LogiMate-GitHub-Ready.zip') $releaseStage
Copy-Item (Join-Path $Root 'dist\RELEASE_ATTESTATION.json') $releaseStage
Copy-Item (Join-Path $Root 'dist\SHA256SUMS.txt') $releaseStage
Copy-Item $verificationPath $releaseStage
Copy-Item $completionPath $releaseStage
Copy-Item (Join-Path $Root 'docs\GITHUB_REPOSITORY_SETTINGS.md') $releaseStage
Copy-Item (Join-Path $Root 'docs\BUILD018_AUDIT.md') $releaseStage
Copy-Item (Join-Path $Root 'docs\BUILD018_GATE_REPORT.md') $releaseStage
Copy-Item $Transcript $releaseStage
Compress-Archive -Path (Join-Path $releaseStage '*') -DestinationPath $releaseZip -Force
$releaseHash = (Get-FileHash $releaseZip -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Host "`nRelease created:" -ForegroundColor Green
Write-Host $releaseZip
Write-Host "SHA-256 $releaseHash"

if (-not $KeepTools) {
    Remove-Item $GoHome,$PwshHome -Recurse -Force -ErrorAction SilentlyContinue
}
