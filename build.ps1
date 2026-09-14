$ErrorActionPreference = 'Stop'
$env:CGO_ENABLED='0'
$env:GOOS='windows'
$env:GOARCH='amd64'
$Version=(Get-Content (Join-Path $PSScriptRoot 'VERSION') -Raw).Trim()
$Build=(Get-Content (Join-Path $PSScriptRoot 'BUILD') -Raw).Trim()
if (-not $Version) { throw 'VERSION is empty' }
if (-not $Build) { throw 'BUILD is empty' }

# Build 019 toolchain gate. Public/reproducible release builds use the exact Go
# patch version declared by go.mod and a current PowerShell 7 build shell.
$GoDirective = (Get-Content (Join-Path $PSScriptRoot 'go.mod') | Where-Object { $_ -match '^go\s+' } | Select-Object -First 1)
if (-not $GoDirective) { throw 'go.mod has no go toolchain directive' }
$ExpectedGo = (($GoDirective -split '\s+')[1]).Trim()
$ActualGo = (& go env GOVERSION).Trim() -replace '^go',''
if ($ActualGo -ne $ExpectedGo) { throw "Go toolchain mismatch: expected $ExpectedGo from go.mod, got $ActualGo" }
$ExpectedPowerShell = [version]'7.6.6'
if ($PSVersionTable.PSVersion -ne $ExpectedPowerShell) { throw "PowerShell toolchain mismatch: expected exactly $ExpectedPowerShell for the Build 019 release pipeline; current=$($PSVersionTable.PSVersion)" }

function Test-StableVersion {
    param([string]$Version)
    return (-not ($Version -match '-'))
}

function Read-CertificationManifest {
    $certPath = Join-Path $PSScriptRoot 'docs/HARDWARE_CERTIFICATION.json'
    if (-not (Test-Path $certPath)) { throw 'Release build requires docs/HARDWARE_CERTIFICATION.json' }
    return (Get-Content $certPath -Raw | ConvertFrom-Json)
}

function Assert-StableCertification {
    param([string]$Version)
    if (-not (Test-StableVersion $Version)) { return }
    $cert = Read-CertificationManifest
    if ($cert.release -ne $Version) { throw "Stable build blocked: certification release '$($cert.release)' does not equal VERSION '$Version'" }
    foreach ($field in @('hardwareMatrixValidated','crashRecoveryValidated','migrationValidated','uiAccessibilityValidated','hidStressValidated')) {
        if ($cert.$field -ne $true) { throw "Stable build blocked: certification field '$field' is not true" }
    }
    foreach ($model in @('g25','g27','dfgt')) {
        if ($null -eq $cert.perModel -or $null -eq $cert.perModel.$model -or $cert.perModel.$model.validated -ne $true) {
            throw "Stable build blocked: model '$model' is not physically certified"
        }
        if ($null -eq $cert.perModel.$model.evidence -or $cert.perModel.$model.evidence.Count -eq 0) {
            throw "Stable build blocked: model '$model' has no certification evidence"
        }
    }
}

function Find-SignTool {
    $cmd = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($cmd) { return $cmd.Source }
    $kits = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    if (Test-Path $kits) {
        $candidate = Get-ChildItem -Path $kits -Directory -ErrorAction SilentlyContinue |
            Sort-Object Name -Descending |
            ForEach-Object { Join-Path $_.FullName 'x64\signtool.exe' } |
            Where-Object { Test-Path $_ } |
            Select-Object -First 1
        if ($candidate) { return $candidate }
    }
    return $null
}

function Get-AuthenticodeRecord {
    param([string]$Path)
    $sig = Get-AuthenticodeSignature -LiteralPath $Path
    $thumb = ''
    $subject = ''
    if ($sig.SignerCertificate) {
        $thumb = $sig.SignerCertificate.Thumbprint
        $subject = $sig.SignerCertificate.Subject
    }
    return [ordered]@{
        status = $sig.Status.ToString()
        subject = $subject
        thumbprint = $thumb
    }
}

function Assert-AuthenticodeValid {
    param([string]$Path,[string]$ExpectedThumbprint)
    $sig = Get-AuthenticodeSignature -LiteralPath $Path
    if ($sig.Status -ne 'Valid' -or $null -eq $sig.SignerCertificate) {
        throw "Authenticode verification failed for '$Path': status=$($sig.Status)"
    }
    $actual = ($sig.SignerCertificate.Thumbprint -replace '\s','').ToUpperInvariant()
    $expected = ($ExpectedThumbprint -replace '\s','').ToUpperInvariant()
    if ($expected -and $actual -ne $expected) {
        throw "Authenticode publisher mismatch for '$Path': actual=$actual expected=$expected"
    }
    return $actual
}

function Resolve-SigningCertificateStore {
    param([string]$Thumbprint)
    $normalized = ($Thumbprint -replace '\s','').ToUpperInvariant()
    $current = Get-ChildItem Cert:\CurrentUser\My -ErrorAction SilentlyContinue | Where-Object { ($_.Thumbprint -replace '\s','').ToUpperInvariant() -eq $normalized } | Select-Object -First 1
    if ($current) {
        if (-not $current.HasPrivateKey) { throw "Signing certificate $normalized in CurrentUser\\My has no private key" }
        return 'CurrentUser'
    }
    $machine = Get-ChildItem Cert:\LocalMachine\My -ErrorAction SilentlyContinue | Where-Object { ($_.Thumbprint -replace '\s','').ToUpperInvariant() -eq $normalized } | Select-Object -First 1
    if ($machine) {
        if (-not $machine.HasPrivateKey) { throw "Signing certificate $normalized in LocalMachine\\My has no private key" }
        return 'LocalMachine'
    }
    throw "Signing certificate $normalized was not found in CurrentUser\\My or LocalMachine\\My"
}

function Invoke-ReleaseSign {
    param([string]$Path,[bool]$Required)
    $thumb = [string]$env:LOGIMATE_SIGN_THUMBPRINT
    $thumb = ($thumb -replace '\s','').Trim()
    $timestamp = [string]$env:LOGIMATE_TIMESTAMP_URL
    $timestamp = $timestamp.Trim()
    if (-not $thumb) {
        if ($Required) { throw 'Stable build blocked: LOGIMATE_SIGN_THUMBPRINT is not configured' }
        Write-Host "Pre-release signing skipped for $Path (LOGIMATE_SIGN_THUMBPRINT not set)."
        return $false
    }
    if (-not $timestamp) {
        if ($Required) { throw 'Stable build blocked: LOGIMATE_TIMESTAMP_URL is not configured' }
        throw 'Signing was requested but LOGIMATE_TIMESTAMP_URL is not configured'
    }
    $signtool = Find-SignTool
    if (-not $signtool) { throw 'signtool.exe not found; install the Windows SDK signing tools' }
    $store = Resolve-SigningCertificateStore -Thumbprint $thumb
    $signArgs = @('sign','/sha1',$thumb,'/fd','SHA256','/tr',$timestamp,'/td','SHA256','/d','LogiMate')
    if ($store -eq 'LocalMachine') { $signArgs += '/sm' }
    $signArgs += $Path
    & $signtool @signArgs | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "signtool sign failed for '$Path' with exit code $LASTEXITCODE" }
    & $signtool verify /pa /all /v $Path | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "signtool verify failed for '$Path' with exit code $LASTEXITCODE" }
    [void](Assert-AuthenticodeValid -Path $Path -ExpectedThumbprint $thumb)
    return $true
}

function New-ReleaseAttestation {
    param([string]$Version,[string]$Build,[bool]$Stable,[bool]$AppSigned,[bool]$InstallerSigned)
    $cert = Read-CertificationManifest
    $appSig = Get-AuthenticodeRecord -Path 'LogiMate.exe'
    $installerSig = Get-AuthenticodeRecord -Path 'LogiMate-Setup-x64.exe'
    $appHash = (Get-FileHash 'LogiMate.exe' -Algorithm SHA256).Hash.ToLowerInvariant()
    $installerHash = (Get-FileHash 'LogiMate-Setup-x64.exe' -Algorithm SHA256).Hash.ToLowerInvariant()
    $samePublisher = $false
    if ($appSig.thumbprint -and $installerSig.thumbprint) {
        $samePublisher = ($appSig.thumbprint -replace '\s','').ToUpperInvariant() -eq ($installerSig.thumbprint -replace '\s','').ToUpperInvariant()
    }
    $modelsPassed = $true
    $modelSummary = [ordered]@{}
    foreach ($model in @('g25','g27','dfgt')) {
        $node = $cert.perModel.$model
        $evidenceCount = 0
        if ($null -ne $node -and $null -ne $node.evidence) { $evidenceCount = $node.evidence.Count }
        $validated = ($null -ne $node -and $node.validated -eq $true)
        $modelSummary[$model] = [ordered]@{ validated = $validated; evidenceCount = $evidenceCount }
        if (-not $validated -or $evidenceCount -lt 1) { $modelsPassed = $false }
    }
    $globalCertPassed = ([bool]$cert.hardwareMatrixValidated -and [bool]$cert.crashRecoveryValidated -and [bool]$cert.migrationValidated -and [bool]$cert.uiAccessibilityValidated -and [bool]$cert.hidStressValidated)
    $att = [ordered]@{
        schemaVersion = 1
        product = 'LogiMate'
        version = $Version
        build = $Build
        updateIdentity = $(if ($Version -eq '0.0.1-alpha') { "$Version.$([int]$Build)" } else { $Version })
        channel = $(if ($Stable) { 'stable' } else { 'prerelease' })
        generatedAtUtc = [DateTime]::UtcNow.ToString('o')
        signatureRequired = $Stable
        toolchain = [ordered]@{
            goVersion = ((& go version).Trim())
            goEnvVersion = ((& go env GOVERSION).Trim())
            powerShellVersion = $PSVersionTable.PSVersion.ToString()
            requiredGoVersion = $ExpectedGo
            requiredPowerShellVersion = $ExpectedPowerShell.ToString()
        }
        certification = [ordered]@{
            release = $cert.release
            hardwareMatrixValidated = [bool]$cert.hardwareMatrixValidated
            crashRecoveryValidated = [bool]$cert.crashRecoveryValidated
            migrationValidated = [bool]$cert.migrationValidated
            uiAccessibilityValidated = [bool]$cert.uiAccessibilityValidated
            hidStressValidated = [bool]$cert.hidStressValidated
            perModel = $modelSummary
        }
        application = [ordered]@{
            file = 'LogiMate.exe'
            sha256 = $appHash
            signedThisBuild = $AppSigned
            authenticodeStatus = $appSig.status
            subject = $appSig.subject
            thumbprint = $appSig.thumbprint
        }
        installer = [ordered]@{
            file = 'LogiMate-Setup-x64.exe'
            sha256 = $installerHash
            signedThisBuild = $InstallerSigned
            authenticodeStatus = $installerSig.status
            subject = $installerSig.subject
            thumbprint = $installerSig.thumbprint
        }
        samePublisher = $samePublisher
        stableGateSatisfied = ($Stable -and $AppSigned -and $InstallerSigned -and $samePublisher -and $globalCertPassed -and $modelsPassed)
    }
    $att | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 'RELEASE_ATTESTATION.json'
}

$Stable = Test-StableVersion $Version
Assert-StableCertification -Version $Version

# Standalone production invariant: the production executable must not compile against
# the historical OpenG27 reference package. That package remains source/test
# material for provenance and parity only.
$prodDeps = (& go list -deps ./cmd/logimate) -join "`n"
if ($prodDeps -match 'internal/openg27port') {
    throw 'Standalone gate failed: production runtime depends on internal/openg27port'
}

go build -trimpath -ldflags "-H=windowsgui -s -w -X main.version=$Version -X main.buildID=$Build" -o LogiMate.exe ./cmd/logimate
go vet -unsafeptr=false ./...
go test ./...
$smoke = Start-Process .\LogiMate.exe -ArgumentList '--smoke-test' -PassThru -Wait
if ($smoke.ExitCode -ne 0) { throw "LogiMate startup smoke test failed with exit code $($smoke.ExitCode)" }

# Release trust: sign and verify the exact application payload BEFORE it is embedded in
# the installer. Stable cannot continue without a valid publisher signature.
$AppSigned = Invoke-ReleaseSign -Path 'LogiMate.exe' -Required $Stable
if ($Stable) {
    $signedSmoke = Start-Process .\LogiMate.exe -ArgumentList '--smoke-test' -PassThru -Wait
    if ($signedSmoke.ExitCode -ne 0) { throw "Signed LogiMate startup smoke test failed with exit code $($signedSmoke.ExitCode)" }
}

New-Item -ItemType Directory -Force cmd/installer/payload | Out-Null
Copy-Item LogiMate.exe cmd/installer/payload/LogiMate.exe -Force
go vet -unsafeptr=false -tags installer ./cmd/installer
go build -tags installer -trimpath -ldflags "-H=windowsgui -s -w -X main.version=$Version -X main.buildID=$Build" -o LogiMate-Setup-x64.exe ./cmd/installer
Remove-Item cmd/installer/payload/LogiMate.exe -Force -ErrorAction SilentlyContinue

# The installer is signed only after its embedded, already-signed application
# payload is final.
$InstallerSigned = Invoke-ReleaseSign -Path 'LogiMate-Setup-x64.exe' -Required $Stable
if ($Stable) {
    $appThumb = Assert-AuthenticodeValid -Path 'LogiMate.exe' -ExpectedThumbprint $env:LOGIMATE_SIGN_THUMBPRINT
    $installerThumb = Assert-AuthenticodeValid -Path 'LogiMate-Setup-x64.exe' -ExpectedThumbprint $env:LOGIMATE_SIGN_THUMBPRINT
    if ($appThumb -ne $installerThumb) { throw 'Stable build blocked: app and installer are signed by different publisher certificates' }
}

$env:GOARCH='arm64'
go build -trimpath -ldflags "-H=windowsgui -s -w -X main.version=$Version -X main.buildID=$Build" -o LogiMate-arm64-validation.exe ./cmd/logimate
$env:GOARCH='amd64'

New-ReleaseAttestation -Version $Version -Build $Build -Stable $Stable -AppSigned $AppSigned -InstallerSigned $InstallerSigned
if ($Stable) {
    $att = Get-Content 'RELEASE_ATTESTATION.json' -Raw | ConvertFrom-Json
    if ($att.stableGateSatisfied -ne $true) { throw 'Stable build blocked: release attestation is not satisfied' }
}

Remove-Item dist -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force dist/portable | Out-Null
Copy-Item LogiMate.exe dist/portable/
New-Item -ItemType File -Force dist/portable/portable.flag | Out-Null
Copy-Item README.md,LICENSE,THIRD_PARTY_NOTICES.md,CHANGELOG.md,SECURITY.md,STARTUP_TROUBLESHOOTING.md,LogiMate-SafeMode.cmd,VERSION,BUILD,RELEASE_ATTESTATION.json dist/portable/
Compress-Archive dist/portable/* dist/LogiMate-Portable-x64.zip -Force
Copy-Item LogiMate.exe dist/LogiMate.exe -Force
Copy-Item LogiMate-Setup-x64.exe dist/LogiMate-Setup-x64.exe -Force
Copy-Item LogiMate-arm64-validation.exe dist/LogiMate-arm64-validation.exe -Force
Copy-Item RELEASE_ATTESTATION.json dist/RELEASE_ATTESTATION.json -Force

$sourceCommon = @('cmd','internal','docs','go.mod','VERSION','BUILD','README.md','LICENSE','THIRD_PARTY_NOTICES.md','CHANGELOG.md','CONTRIBUTING.md','SECURITY.md','SUPPORT.md','CODE_OF_CONDUCT.md','STARTUP_TROUBLESHOOTING.md','RELEASE_NOTES.txt','BUILD018_RELEASE_KIT.md','SOURCE_MANIFEST_SHA256.txt','build.ps1','Build-Release-Go1271.ps1','Build-Release.cmd','Publish-Repository.ps1','Publish-Repository.cmd','REPOSITORY_SETUP.md','.gitattributes','LogiMate-SafeMode.cmd')
Compress-Archive -Path $sourceCommon -DestinationPath dist/LogiMate-Source.zip -Force
$githubReady = $sourceCommon + @('.github','.gitignore','.editorconfig')
$githubReady = $githubReady | Where-Object { Test-Path $_ }
Compress-Archive -Path $githubReady -DestinationPath dist/LogiMate-GitHub-Ready.zip -Force

Get-FileHash dist/LogiMate.exe,dist/LogiMate-Setup-x64.exe,dist/LogiMate-Portable-x64.zip,dist/LogiMate-arm64-validation.exe,dist/LogiMate-Source.zip,dist/LogiMate-GitHub-Ready.zip,dist/RELEASE_ATTESTATION.json -Algorithm SHA256 |
    ForEach-Object { "$($_.Hash.ToLower())  $([IO.Path]::GetFileName($_.Path))" } |
    Set-Content dist/SHA256SUMS.txt

Write-Host "LogiMate $Version · Build $Build complete. Stable=$Stable AppSigned=$AppSigned InstallerSigned=$InstallerSigned"
