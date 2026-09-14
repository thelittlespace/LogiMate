[CmdletBinding()]
param(
    [ValidateSet('public','private')]
    [string]$Visibility = 'public',
    [switch]$SkipTag
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2.0
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $Root

$Owner = 'thelittlespace'
$Repo = 'LogiMate'
$FullRepo = "$Owner/$Repo"
$ExpectedVersion = '0.0.1-alpha'
$ExpectedBuild = '018'
$ExpectedTag = "v$ExpectedVersion.$ExpectedBuild"

function Require-Command([string]$Name) {
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if (-not $cmd) { throw "$Name was not found. Install it first and rerun this script." }
    return $cmd.Source
}

function Invoke-NativePassthru {
    param(
        [Parameter(Mandatory=$true)][string]$FilePath,
        [Parameter(Mandatory=$false)][string[]]$ArgumentList = @(),
        [switch]$IgnoreExitCode
    )

    # Windows PowerShell 5.1 may turn harmless native STDERR text into a
    # NativeCommandError when $ErrorActionPreference is Stop. Start-Process
    # keeps native stdout/stderr native and lets us judge only the real exit code.
    $process = Start-Process -FilePath $FilePath -ArgumentList $ArgumentList -Wait -PassThru -NoNewWindow
    if (-not $IgnoreExitCode -and $process.ExitCode -ne 0) {
        throw "Native command failed with exit code $($process.ExitCode): $FilePath $($ArgumentList -join ' ')"
    }
    return [int]$process.ExitCode
}

function Invoke-NativeCapture {
    param(
        [Parameter(Mandatory=$true)][string]$FilePath,
        [Parameter(Mandatory=$false)][string[]]$ArgumentList = @()
    )

    $stdoutPath = [System.IO.Path]::GetTempFileName()
    $stderrPath = [System.IO.Path]::GetTempFileName()
    try {
        $process = Start-Process -FilePath $FilePath -ArgumentList $ArgumentList -Wait -PassThru -NoNewWindow `
            -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
        $stdout = [System.IO.File]::ReadAllText($stdoutPath)
        $stderr = [System.IO.File]::ReadAllText($stderrPath)
        return [PSCustomObject]@{
            ExitCode = [int]$process.ExitCode
            StdOut   = $stdout
            StdErr   = $stderr
        }
    } finally {
        Remove-Item -LiteralPath $stdoutPath,$stderrPath -Force -ErrorAction SilentlyContinue
    }
}

function Ensure-GitHubAuthentication {
    Write-Host 'Checking GitHub CLI authentication...' -ForegroundColor Cyan
    $statusCode = Invoke-NativePassthru -FilePath 'gh.exe' -ArgumentList @('auth','status','-h','github.com') -IgnoreExitCode
    $authenticated = ($statusCode -eq 0)

    if (-not $authenticated) {
        Write-Warning 'The stored GitHub CLI login is missing, expired, or invalid.'
        Write-Host "Removing any stale login for $Owner..." -ForegroundColor Yellow
        $null = Invoke-NativePassthru -FilePath 'gh.exe' -ArgumentList @('auth','logout','-h','github.com','-u',$Owner) -IgnoreExitCode
        # A missing/stale account may make logout return non-zero. That is harmless.

        Write-Host ''
        Write-Host 'A GitHub browser login will open now.' -ForegroundColor Cyan
        Write-Host "Sign in with the '$Owner' account and approve the requested permissions." -ForegroundColor Cyan
        Write-Host 'Required scopes: repo + workflow.' -ForegroundColor DarkGray
        $loginCode = Invoke-NativePassthru -FilePath 'gh.exe' -ArgumentList @('auth','login','-h','github.com','--web','--git-protocol','https','--scopes','repo,workflow') -IgnoreExitCode
        if ($loginCode -ne 0) {
            throw 'GitHub browser authentication failed or was cancelled. Rerun Publish-Repository.cmd to try again.'
        }

        $statusCode = Invoke-NativePassthru -FilePath 'gh.exe' -ArgumentList @('auth','status','-h','github.com') -IgnoreExitCode
        if ($statusCode -ne 0) {
            throw 'GitHub CLI still reports an invalid login after authentication.'
        }
    }

    $who = Invoke-NativeCapture -FilePath 'gh.exe' -ArgumentList @('api','user','--jq','.login')
    $login = $who.StdOut.Trim()
    if ($who.ExitCode -ne 0 -or -not $login) {
        throw 'GitHub authentication succeeded, but the active account could not be identified.'
    }
    if ($login -ne $Owner) {
        throw "Safety stop: GitHub CLI is authenticated as '$login', but this release must be published as '$Owner'."
    }

    $setupCode = Invoke-NativePassthru -FilePath 'gh.exe' -ArgumentList @('auth','setup-git') -IgnoreExitCode
    if ($setupCode -ne 0) {
        throw 'GitHub authentication is valid, but gh auth setup-git failed.'
    }

    Write-Host "GitHub CLI authenticated as $login." -ForegroundColor Green
    return $login
}

Require-Command 'git' | Out-Null
Require-Command 'gh' | Out-Null

$version = (Get-Content VERSION -Raw).Trim()
$build = (Get-Content BUILD -Raw).Trim()
if ($version -ne $ExpectedVersion -or $build -ne $ExpectedBuild) {
    throw "Repository identity mismatch. Expected $ExpectedVersion Build $ExpectedBuild, got $version Build $build"
}
if ((Get-Content go.mod -Raw) -notmatch '(?m)^go\s+1\.27\.1\s*$') {
    throw 'go.mod is not pinned to Go 1.27.1'
}

$activeLogin = Ensure-GitHubAuthentication

if (-not (Test-Path .git)) {
    Write-Host 'Initializing local Git repository...' -ForegroundColor Cyan
    $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('init','-b','main') -IgnoreExitCode
    if ($code -ne 0) {
        # Compatibility fallback for older Git versions that do not support init -b.
        $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('init') -IgnoreExitCode
        if ($code -ne 0) { throw 'git init failed' }
        $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('checkout','-B','main') -IgnoreExitCode
        if ($code -ne 0) { throw 'Could not create local main branch' }
    }
}

$nameResult = Invoke-NativeCapture -FilePath 'git.exe' -ArgumentList @('config','user.name')
$userName = $nameResult.StdOut.Trim()
$emailResult = Invoke-NativeCapture -FilePath 'git.exe' -ArgumentList @('config','user.email')
$userEmail = $emailResult.StdOut.Trim()

if (-not $userName) {
    $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('config','user.name',$activeLogin) -IgnoreExitCode
    if ($code -ne 0) { throw 'Could not configure local Git user.name' }
    $userName = $activeLogin
}
if (-not $userEmail) {
    $idResult = Invoke-NativeCapture -FilePath 'gh.exe' -ArgumentList @('api','user','--jq','.id')
    $githubId = $idResult.StdOut.Trim()
    if ($idResult.ExitCode -ne 0 -or -not $githubId) { throw 'Could not determine GitHub account id for noreply commit email.' }
    $userEmail = "$githubId+$activeLogin@users.noreply.github.com"
    $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('config','user.email',$userEmail) -IgnoreExitCode
    if ($code -ne 0) { throw 'Could not configure local Git user.email' }
    Write-Host "Configured repository-local Git author: $userName <$userEmail>" -ForegroundColor DarkGray
}

$code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('add','--all') -IgnoreExitCode
if ($code -ne 0) { throw 'git add failed' }

$headResult = Invoke-NativeCapture -FilePath 'git.exe' -ArgumentList @('rev-parse','--verify','HEAD')
$hasHead = ($headResult.ExitCode -eq 0)
if (-not $hasHead) {
    $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('commit','-m','LogiMate-Build-018') -IgnoreExitCode
    if ($code -ne 0) { throw 'Initial commit failed' }
} else {
    $diffResult = Invoke-NativeCapture -FilePath 'git.exe' -ArgumentList @('diff','--cached','--quiet')
    if ($diffResult.ExitCode -ne 0) {
        $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('commit','-m','Prepare-Build-018-release') -IgnoreExitCode
        if ($code -ne 0) { throw 'Commit failed' }
    }
}

$repoView = Invoke-NativeCapture -FilePath 'gh.exe' -ArgumentList @('repo','view',$FullRepo,'--json','name')
$repoExists = ($repoView.ExitCode -eq 0)

if (-not $repoExists) {
    $visibilityArg = if ($Visibility -eq 'public') { '--public' } else { '--private' }
    Write-Host "Creating GitHub repository $FullRepo ($Visibility)..." -ForegroundColor Cyan
    $code = Invoke-NativePassthru -FilePath 'gh.exe' -ArgumentList @('repo','create',$FullRepo,$visibilityArg,'--source','.','--remote','origin','--push') -IgnoreExitCode
    if ($code -ne 0) { throw 'GitHub repository creation failed' }
} else {
    $originResult = Invoke-NativeCapture -FilePath 'git.exe' -ArgumentList @('remote','get-url','origin')
    $origin = $originResult.StdOut.Trim()
    $expectedHttps = "https://github.com/$FullRepo.git"
    $expectedSsh = "git@github.com:$FullRepo.git"
    if (-not $origin) {
        $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('remote','add','origin',$expectedHttps) -IgnoreExitCode
        if ($code -ne 0) { throw 'Could not add the expected GitHub origin.' }
    } elseif ($origin -ne $expectedHttps -and $origin -ne $expectedSsh) {
        throw "Safety stop: existing origin '$origin' is not $FullRepo"
    }
    $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('push','-u','origin','main') -IgnoreExitCode
    if ($code -ne 0) { throw 'Push to main failed' }
}

# Safe repository defaults. Failure here is non-fatal because some GitHub
# account/app configurations may restrict repository administration APIs.
$settings = Invoke-NativeCapture -FilePath 'gh.exe' -ArgumentList @(
    'api','--method','PATCH',"repos/$FullRepo",
    '-F','allow_squash_merge=true',
    '-F','allow_merge_commit=false',
    '-F','allow_rebase_merge=false',
    '-F','delete_branch_on_merge=true',
    '-F','has_issues=true'
)
if ($settings.ExitCode -ne 0) {
    Write-Warning 'Repository merge/default settings could not be updated automatically.'
}

if (-not $SkipTag) {
    $tagResult = Invoke-NativeCapture -FilePath 'git.exe' -ArgumentList @('rev-parse','--verify',$ExpectedTag)
    if ($tagResult.ExitCode -ne 0) {
        $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('tag','-a',$ExpectedTag,'-m','LogiMate-Build-018') -IgnoreExitCode
        if ($code -ne 0) { throw 'Release tag creation failed' }
    }
    $code = Invoke-NativePassthru -FilePath 'git.exe' -ArgumentList @('push','origin',$ExpectedTag) -IgnoreExitCode
    if ($code -ne 0) { throw 'Release tag push failed' }
    Write-Host "Release workflow triggered by $ExpectedTag" -ForegroundColor Green
}

Write-Host "Repository ready: https://github.com/$FullRepo" -ForegroundColor Green
Write-Host 'The tag-triggered GitHub Actions workflow builds Go 1.27.1 artifacts and publishes a prerelease after all required gates pass.'
