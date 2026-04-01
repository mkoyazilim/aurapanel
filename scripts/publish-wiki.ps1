param(
    [string]$Owner = "mkoyazilim",
    [string]$Repo = "aurapanel",
    [string]$SourceDir = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Continue"

function Write-Step {
    param([string]$Message)
    Write-Host "[publish-wiki] $Message" -ForegroundColor Cyan
}

if ([string]::IsNullOrWhiteSpace($SourceDir)) {
    $SourceDir = (Join-Path $PSScriptRoot "..\wiki")
}

if (-not (Test-Path $SourceDir)) {
    throw "Wiki source directory not found: $SourceDir"
}
$SourceDir = (Resolve-Path $SourceDir).Path

$wikiRemote = "https://github.com/$Owner/$Repo.wiki.git"
$repoBlobBase = "https://github.com/$Owner/$Repo/blob/main"
$tempDir = Join-Path $env:TEMP ("aurapanel-wiki-" + [Guid]::NewGuid().ToString("N"))
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

try {
    Write-Step "Cloning wiki repository: $wikiRemote"
    $cloneOutput = & git clone $wikiRemote $tempDir 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        $text = $cloneOutput
        if ($text -match "Repository not found") {
            throw @"
GitHub Wiki git repository is not initialized yet.
Please open this page once and create the first wiki page:
https://github.com/$Owner/$Repo/wiki

After that, run:
powershell -ExecutionPolicy Bypass -File scripts/publish-wiki.ps1
"@
        }
        throw "Failed to clone wiki repository.`n$text"
    }

    Write-Step "Preparing markdown pages from $SourceDir"
    $sourceFiles = Get-ChildItem -Path $SourceDir -File -Filter "*.md"
    if (-not $sourceFiles) {
        throw "No markdown files found in $SourceDir"
    }

    foreach ($file in $sourceFiles) {
        $content = Get-Content -Path $file.FullName -Raw

        # Wiki-compatible page links: ./Page.md -> Page
        $content = [Regex]::Replace(
            $content,
            '\]\(\./([^)#]+?)\.md\)',
            ']($1)'
        )
        $content = [Regex]::Replace(
            $content,
            '\]\(\./([^)#]+?)\.md#([^)]+)\)',
            ']($1#$2)'
        )

        # Repository-relative references: ../README.md -> absolute blob URL
        $content = [Regex]::Replace(
            $content,
            '\]\(\.\./([^)]+)\)',
            {
                param($m)
                $path = $m.Groups[1].Value
                return "]($repoBlobBase/$path)"
            }
        )

        $targetPath = Join-Path $tempDir $file.Name
        [System.IO.File]::WriteAllText($targetPath, $content, $utf8NoBom)
    }

    Push-Location $tempDir
    try {
        Write-Step "Checking for wiki changes"
        & git add -A
        if ($LASTEXITCODE -ne 0) {
            throw "git add failed"
        }

        & git diff --cached --quiet
        if ($LASTEXITCODE -eq 0) {
            Write-Step "No wiki changes to publish."
            return
        }

        $commitMessage = "docs(wiki): sync wiki pages from repository seed"
        Write-Step "Committing: $commitMessage"
        & git commit -m $commitMessage
        if ($LASTEXITCODE -ne 0) {
            throw "git commit failed"
        }

        Write-Step "Pushing to GitHub Wiki"
        & git push origin HEAD
        if ($LASTEXITCODE -ne 0) {
            throw "git push failed"
        }
    }
    finally {
        Pop-Location
    }

    Write-Step "Wiki publish completed successfully."
}
finally {
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force
    }
}
