# Draws the desktop shortcut icon embedded in the Engine. Needs Windows PowerShell 5.1
# (System.Drawing): powershell.exe -File scripts\make-shortcut-icon.ps1
param(
    [string]$Out = (Join-Path $PSScriptRoot '..\internal\platform\assets\CanvasExpert.ico'),
    # Optional folder for one PNG per size, to look at the result.
    [string]$PreviewDir
)
Add-Type -AssemblyName System.Drawing
$ErrorActionPreference = 'Stop'

function RoundRect([float]$x, [float]$y, [float]$w, [float]$h, [float]$r) {
    $p = New-Object System.Drawing.Drawing2D.GraphicsPath
    $d = 2 * $r
    $p.AddArc($x, $y, $d, $d, 180, 90)
    $p.AddArc($x + $w - $d, $y, $d, $d, 270, 90)
    $p.AddArc($x + $w - $d, $y + $h - $d, $d, $d, 0, 90)
    $p.AddArc($x, $y + $h - $d, $d, $d, 90, 90)
    $p.CloseFigure()
    return $p
}
function C([string]$hex, [int]$a = 255) {
    $c = [System.Drawing.ColorTranslator]::FromHtml($hex)
    return [System.Drawing.Color]::FromArgb($a, $c.R, $c.G, $c.B)
}
function Star([float]$cx, [float]$cy, [float]$Router, [float]$Rinner) {
    $pts = @()
    for ($i = 0; $i -lt 8; $i++) {
        $ang = [math]::PI / 4 * $i - [math]::PI / 2
        $rad = if ($i % 2 -eq 0) { $Router } else { $Rinner }
        $pts += New-Object System.Drawing.PointF(($cx + $rad * [math]::Cos($ang)), ($cy + $rad * [math]::Sin($ang)))
    }
    return , $pts
}

function Render([int]$S) {
    $bmp = New-Object System.Drawing.Bitmap($S, $S, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'; $g.PixelOffsetMode = 'HighQuality'; $g.CompositingQuality = 'HighQuality'
    $g.Clear([System.Drawing.Color]::Transparent)
    $u = $S / 256.0
    $small = $S -le 24

    # Tile: deep teal with a diagonal sheen.
    $pad = if ($small) { 0.5 } else { 8 * $u }
    $tile = RoundRect $pad $pad ($S - 2 * $pad) ($S - 2 * $pad) (52 * $u)
    $grad = New-Object System.Drawing.Drawing2D.LinearGradientBrush((New-Object System.Drawing.PointF(0, 0)), (New-Object System.Drawing.PointF($S, $S)), (C '#2f9488'), (C '#134a45'))
    $g.FillPath($grad, $tile)
    if (-not $small) {
        $g.SetClip($tile)
        $sheen = New-Object System.Drawing.SolidBrush((C '#ffffff' 22))
        $g.FillEllipse($sheen, -90 * $u, -150 * $u, 330 * $u, 260 * $u)
        $g.ResetClip()
    }

    # Canvas card: white page with a mint header strip.
    if ($small) { $cx = 3.5 * $S / 16; $cy = 3 * $S / 16; $cw = 9.5 * $S / 16; $ch = 10.5 * $S / 16 }
    else { $cx = 58 * $u; $cy = 50 * $u; $cw = 140 * $u; $ch = 158 * $u }
    if (-not $small) {
        $shadow = RoundRect ($cx + 6 * $u) ($cy + 10 * $u) $cw $ch (18 * $u)
        $g.FillPath((New-Object System.Drawing.SolidBrush((C '#04211e' 70))), $shadow)
    }
    $card = RoundRect $cx $cy $cw $ch ([math]::Max(1.5, 18 * $u))
    $g.FillPath((New-Object System.Drawing.SolidBrush((C '#ffffff'))), $card)
    if (-not $small) {
        $g.SetClip($card)
        $g.FillRectangle((New-Object System.Drawing.SolidBrush((C '#a5c5b2'))), $cx, $cy, $cw, 30 * $u)
        $g.ResetClip()
        # Ruled lines
        $line = New-Object System.Drawing.Pen((C '#cdded4'), (9 * $u)); $line.StartCap = 'Round'; $line.EndCap = 'Round'
        $g.DrawLine($line, $cx + 24 * $u, $cy + 62 * $u, $cx + $cw - 24 * $u, $cy + 62 * $u)
        $g.DrawLine($line, $cx + 24 * $u, $cy + 88 * $u, $cx + $cw - 56 * $u, $cy + 88 * $u)
    }

    # Big check mark, bleeding past the card for energy.
    $w = if ($small) { [math]::Max(2.2, 2.6 * $S / 16) } else { 30 * $u }
    $pen = New-Object System.Drawing.Pen((C '#216c65'), $w)
    $pen.StartCap = 'Round'; $pen.EndCap = 'Round'; $pen.LineJoin = 'Round'
    if ($small) {
        $pts = @((New-Object System.Drawing.PointF((5.6 * $S / 16), (8.6 * $S / 16))), (New-Object System.Drawing.PointF((7.6 * $S / 16), (10.6 * $S / 16))), (New-Object System.Drawing.PointF((11 * $S / 16), (6 * $S / 16))))
    } else {
        $pts = @((New-Object System.Drawing.PointF((92 * $u), (150 * $u))), (New-Object System.Drawing.PointF((124 * $u), (182 * $u))), (New-Object System.Drawing.PointF((214 * $u), (92 * $u))))
        $halo = New-Object System.Drawing.Pen((C '#ffffff'), ($w + 18 * $u)); $halo.StartCap = 'Round'; $halo.EndCap = 'Round'; $halo.LineJoin = 'Round'
        $g.DrawLines($halo, [System.Drawing.PointF[]]$pts)
    }
    $g.DrawLines($pen, [System.Drawing.PointF[]]$pts)

    # Gold spark: the "expert" touch.
    if ($S -ge 32) {
        $spark = Star (204 * $u) (52 * $u) (34 * $u) (11 * $u)
        $g.FillPolygon((New-Object System.Drawing.SolidBrush((C '#f6c453'))), [System.Drawing.PointF[]]$spark)
        $spark2 = Star (226 * $u) (100 * $u) (14 * $u) (5 * $u)
        $g.FillPolygon((New-Object System.Drawing.SolidBrush((C '#ffe7a3'))), [System.Drawing.PointF[]]$spark2)
    }
    $g.Dispose()
    return $bmp
}

$sizes = 16, 20, 24, 32, 40, 48, 64, 128, 256
$pngs = @()
foreach ($s in $sizes) {
    $bmp = Render $s
    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    if ($PreviewDir) { $bmp.Save((Join-Path $PreviewDir "icon-$s.png"), [System.Drawing.Imaging.ImageFormat]::Png) }
    $bmp.Dispose()
    $pngs += , $ms.ToArray()
}

# ICO container with PNG-compressed entries (supported since Windows Vista).
$fs = [System.IO.File]::Create($Out)
$bw = New-Object System.IO.BinaryWriter($fs)
$bw.Write([uint16]0); $bw.Write([uint16]1); $bw.Write([uint16]$sizes.Count)
$offset = 6 + 16 * $sizes.Count
for ($i = 0; $i -lt $sizes.Count; $i++) {
    $s = $sizes[$i]; $b = if ($s -ge 256) { 0 } else { $s }
    $bw.Write([byte]$b); $bw.Write([byte]$b); $bw.Write([byte]0); $bw.Write([byte]0)
    $bw.Write([uint16]1); $bw.Write([uint16]32)
    $bw.Write([uint32]$pngs[$i].Length); $bw.Write([uint32]$offset)
    $offset += $pngs[$i].Length
}
foreach ($p in $pngs) { $bw.Write($p) }
$bw.Close()

