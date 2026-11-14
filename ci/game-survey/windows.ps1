# Game survey on a GitHub Windows runner: download one game from games.json,
# check its SHA-256, extract it, write its config, launch it windowed and take
# desktop screenshots at the times the spec lists.
#
# Writes screenshots, logs and a result.txt to $OutDir. Never copies game files
# there: the folder is uploaded as a public artifact.
param(
  [Parameter(Mandatory)] [string] $Game,
  [Parameter(Mandatory)] [string] $OutDir
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$all = Get-Content -Raw "$PSScriptRoot/games.json" | ConvertFrom-Json
$spec = $all.games.$Game
if (-not $spec) { throw "unknown game $Game" }
if ($spec.same_as) {
  $base = $all.games.($spec.same_as)
  foreach ($p in $spec.PSObject.Properties) {
    if ($p.Name -ne 'same_as') { $base | Add-Member -Force -NotePropertyName $p.Name -NotePropertyValue $p.Value }
  }
  $spec = $base
}

$work = 'C:\gs'
$dl = "$work\dl"
$root = "$work\game"
New-Item -ItemType Directory -Force $dl, $root, $OutDir | Out-Null
$sevenZip = 'C:\Program Files\7-Zip\7z.exe'

function Log($msg) {
  $line = "$(Get-Date -Format HH:mm:ss) $msg"
  Write-Host $line
  Add-Content "$OutDir\result.txt" $line
}

function Fetch($url, $sha) {
  $file = Join-Path $dl ([IO.Path]::GetFileName(([Uri]$url).AbsolutePath))
  if (-not (Test-Path $file)) {
    & curl.exe -fsSL --retry 3 -o $file $url
    if ($LASTEXITCODE) { throw "download failed: $url" }
  }
  $got = (Get-FileHash -Algorithm SHA256 $file).Hash.ToLower()
  if ($got -ne $sha) { throw "SHA-256 mismatch for $url`: got $got, want $sha" }
  Log "downloaded $url ($((Get-Item $file).Length) bytes, sha256 ok)"
  return $file
}

function Expand-Path($p) {
  $p = $p.Replace('{game}', $root).Replace('{appdata}', $env:APPDATA)
  $p = $p.Replace('{documents}', [Environment]::GetFolderPath('MyDocuments'))
  return $p.Replace('/', '\')
}

function Screenshot($path) {
  Add-Type -AssemblyName System.Windows.Forms, System.Drawing
  $b = [System.Windows.Forms.SystemInformation]::VirtualScreen
  $bmp = New-Object System.Drawing.Bitmap $b.Width, $b.Height
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.CopyFromScreen($b.Location, [System.Drawing.Point]::Empty, $b.Size)
  $bmp.Save($path, [System.Drawing.Imaging.ImageFormat]::Png)
  $g.Dispose(); $bmp.Dispose()
}

$t0 = Get-Date
foreach ($d in $spec.downloads) {
  $file = Fetch $d.url $d.sha256
  $into = Join-Path $root $d.into
  New-Item -ItemType Directory -Force $into | Out-Null
  switch ($d.extract) {
    { $_ -in 'zip', '7z' } {
      & $sevenZip x -y "-o$into" $file | Out-Null
      if ($LASTEXITCODE) { throw "7z failed on $file" }
    }
    'inno-silent' {
      # Inno Setup installs with no window. An "inner" file means the download
      # is a 7z wrapper around the real Inno Setup exe.
      $setup = $file
      if ($d.inner) {
        & $sevenZip x -y "-o$dl\setup" $file | Out-Null
        $setup = "$dl\setup\$($d.inner)"
      }
      $p = Start-Process -FilePath $setup -Wait -PassThru -ArgumentList '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-', "/DIR=$into"
      Log "installer exit code $($p.ExitCode)"
    }
    default { throw "unknown extract method $($d.extract)" }
  }
}
Log "extracted in $([int]((Get-Date) - $t0).TotalSeconds)s"

$exe = Join-Path $root $spec.exe
if (-not (Test-Path $exe)) {
  Get-ChildItem -Recurse -Depth 3 $root -Filter *.exe | ForEach-Object { Log "found exe: $($_.FullName)" }
  throw "exe not found: $exe"
}
$cwd = if ($spec.cwd) { Join-Path $root $spec.cwd } else { Split-Path $exe }

foreach ($f in $spec.files) {
  $to = Expand-Path $f.to
  New-Item -ItemType Directory -Force (Split-Path $to) | Out-Null
  Copy-Item "$PSScriptRoot/config/$($f.from)" $to -Force
  Log "wrote $to"
}

if ($spec.opengl) {
  # Windows on the runner has no OpenGL driver beyond GDI's 1.1, so put Mesa's
  # llvmpipe next to the exe.
  $mesa = Fetch $all.mesa.url $all.mesa.sha256
  $mesaDir = "$work\mesa"
  & $sevenZip x -y "-o$mesaDir" $mesa @($all.mesa.files) | Out-Null
  foreach ($m in $all.mesa.files) { Copy-Item (Join-Path $mesaDir $m) (Split-Path $exe) -Force }
  $env:GALLIUM_DRIVER = 'llvmpipe'
  Log 'Mesa llvmpipe opengl32.dll placed next to the exe'
}

Screenshot "$OutDir\00-before.png"
$argList = @($spec.args | ForEach-Object { if ($_ -match '\s') { "`"$_`"" } else { $_ } })
Log "launch: $exe $($argList -join ' ') (cwd $cwd)"
$start = Get-Date
$proc = if ($argList.Count) {
  Start-Process -FilePath $exe -ArgumentList $argList -WorkingDirectory $cwd -PassThru
} else {
  Start-Process -FilePath $exe -WorkingDirectory $cwd -PassThru
}

foreach ($s in $spec.shots) {
  $wait = $s - ((Get-Date) - $start).TotalSeconds
  if ($wait -gt 0) { Start-Sleep -Seconds $wait }
  $name = '{0:D3}s.png' -f [int]$s
  Screenshot "$OutDir\$name"
  $alive = -not $proc.HasExited
  $title = ''
  if ($alive) { $proc.Refresh(); $title = $proc.MainWindowTitle }
  Log "shot $name alive=$alive window='$title'"
  if (-not $alive) { Log "exited with code $($proc.ExitCode)"; break }
}

Get-Process | Where-Object { $_.MainWindowTitle } | ForEach-Object { Log "window: $($_.ProcessName) '$($_.MainWindowTitle)'" }
if (-not $proc.HasExited) { Stop-Process -Id $proc.Id -Force }

foreach ($l in $spec.logs) {
  Get-ChildItem (Expand-Path $l) -ErrorAction SilentlyContinue | ForEach-Object {
    Copy-Item $_.FullName (Join-Path $OutDir $_.Name)
  }
}
Log 'done'
