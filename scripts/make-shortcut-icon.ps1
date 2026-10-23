# Draws the desktop shortcut icon embedded in the Engine: a retro robot brain, half
# organic and half circuitry, under a glass dome on a riveted base. Needs Windows
# PowerShell 5.1 (System.Drawing): powershell.exe -File scripts\make-shortcut-icon.ps1
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
function Brush([string]$hex, [int]$a = 255) { New-Object System.Drawing.SolidBrush((C $hex $a)) }
function RoundPen([string]$hex, [float]$w, [int]$a = 255) {
    $p = New-Object System.Drawing.Pen((C $hex $a), $w)
    $p.StartCap = 'Round'; $p.EndCap = 'Round'; $p.LineJoin = 'Round'
    return $p
}
function Pt([float]$x, [float]$y) { New-Object System.Drawing.PointF($x, $y) }

# The brain silhouette as overlapping lobes, in 256-unit space.
function BrainPath([float]$cx, [float]$cy, [float]$k) {
    $p = New-Object System.Drawing.Drawing2D.GraphicsPath
    $p.FillMode = 'Winding'
    foreach ($l in @(@(-50, -22, 52, 46), @(-18, -38, 58, 50), @(18, -26, 50, 46),
                     @(-58, 2, 56, 44), @(-12, -2, 60, 46), @(22, 0, 46, 42))) {
        $p.AddEllipse($cx + $l[0] * $k, $cy + $l[1] * $k, $l[2] * $k, $l[3] * $k)
    }
    return $p
}

function DrawBrain($g, [float]$cx, [float]$cy, [float]$k, [bool]$detail) {
    $brain = BrainPath $cx $cy $k
    # Stroke every lobe, then fill over the inner strokes: only the silhouette stays.
    $outline = RoundPen '#5a1f2b' (($(if ($detail) { 16 } else { 24 })) * $k)
    $g.DrawPath($outline, $brain)
    $g.FillPath((Brush '#f2878f'), $brain)

    # Right hemisphere is machine: chrome with circuit traces.
    $g.SetClip($brain)
    $g.SetClip((New-Object System.Drawing.RectangleF(($cx + 2 * $k), ($cy - 60 * $k), (90 * $k), (120 * $k))), 'Intersect')
    $chrome = New-Object System.Drawing.Drawing2D.LinearGradientBrush((Pt $cx ($cy - 40 * $k)), (Pt $cx ($cy + 46 * $k)), (C '#e9f4f1'), (C '#8fb5ae'))
    $g.FillPath($chrome, $brain)
    if ($detail) {
        $trace = RoundPen '#216c65' (5 * $k)
        $gold = Brush '#f6c453'
        foreach ($t in @(@(@(4, -24), @(22, -24), @(34, -12), @(52, -12)),
                         @(@(4, 0), @(28, 0), @(40, 12), @(60, 12)),
                         @(@(4, 22), @(18, 22), @(30, 34)))) {
            $pts = [System.Drawing.PointF[]]@($t | ForEach-Object { Pt ($cx + $_[0] * $k) ($cy + $_[1] * $k) })
            $g.DrawLines($trace, $pts)
            $end = $pts[$pts.Length - 1]
            $r = 6 * $k
            $g.FillEllipse($gold, $end.X - $r, $end.Y - $r, 2 * $r, 2 * $r)
            $g.DrawEllipse((RoundPen '#216c65' (3 * $k)), $end.X - $r, $end.Y - $r, 2 * $r, 2 * $r)
        }
    }
    $g.ResetClip()

    # Left hemisphere folds.
    if ($detail) {
        $fold = RoundPen '#c4566a' (6 * $k)
        $g.DrawBezier($fold, (Pt ($cx - 40 * $k) ($cy - 14 * $k)), (Pt ($cx - 30 * $k) ($cy - 30 * $k)), (Pt ($cx - 16 * $k) ($cy - 6 * $k)), (Pt ($cx - 10 * $k) ($cy - 24 * $k)))
        $g.DrawBezier($fold, (Pt ($cx - 48 * $k) ($cy + 16 * $k)), (Pt ($cx - 36 * $k) ($cy + 2 * $k)), (Pt ($cx - 24 * $k) ($cy + 26 * $k)), (Pt ($cx - 12 * $k) ($cy + 10 * $k)))
        $g.DrawBezier($fold, (Pt ($cx - 30 * $k) ($cy + 36 * $k)), (Pt ($cx - 22 * $k) ($cy + 28 * $k)), (Pt ($cx - 14 * $k) ($cy + 40 * $k)), (Pt ($cx - 6 * $k) ($cy + 32 * $k)))
    }
    # Center fissure.
    $g.DrawBezier((RoundPen '#5a1f2b' ($(if ($detail) { 6 } else { 10 }) * $k)), (Pt ($cx + 2 * $k) ($cy - 36 * $k)), (Pt ($cx - 6 * $k) ($cy - 10 * $k)), (Pt ($cx + 8 * $k) ($cy + 14 * $k)), (Pt ($cx + 2 * $k) ($cy + 42 * $k)))
}

function Render([int]$S) {
    $bmp = New-Object System.Drawing.Bitmap($S, $S, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = 'AntiAlias'; $g.PixelOffsetMode = 'HighQuality'; $g.CompositingQuality = 'HighQuality'
    $g.Clear([System.Drawing.Color]::Transparent)
    $g.ScaleTransform(($S / 256.0), ($S / 256.0))
    $small = $S -le 24
    $detail = $S -ge 48

    # Tile: deep teal with a soft radial glow behind the dome.
    $pad = if ($small) { 4 } else { 8 }
    $tile = RoundRect $pad $pad (256 - 2 * $pad) (256 - 2 * $pad) 52
    $grad = New-Object System.Drawing.Drawing2D.LinearGradientBrush((Pt 0 0), (Pt 256 256), (C '#2f9488'), (C '#0f3f3b'))
    $g.FillPath($grad, $tile)
    if (-not $small) {
        $g.SetClip($tile)
        $glow = New-Object System.Drawing.Drawing2D.GraphicsPath
        $glow.AddEllipse(20, 10, 216, 200)
        $pgb = New-Object System.Drawing.Drawing2D.PathGradientBrush($glow)
        $pgb.CenterColor = (C '#7fe0cf' 90); $pgb.SurroundColors = [System.Drawing.Color[]]@((C '#7fe0cf' 0))
        $g.FillPath($pgb, $glow)
        # Retro scan lines.
        if ($detail) {
            $scan = Brush '#000000' 18
            for ($y = 14; $y -lt 250; $y += 10) { $g.FillRectangle($scan, 0, $y, 256, 4) }
        }
        $g.ResetClip()
    }

    if ($small) {
        # Tiny sizes: just the two-tone brain, big.
        DrawBrain $g 128 134 1.75 $false
        $g.Dispose(); return $bmp
    }

    # Antenna with a glowing bulb.
    $g.DrawLine((RoundPen '#f4ead5' 8), 128, 52, 128, 28)
    $g.FillEllipse((Brush '#f6c453' 90), 108, 4, 40, 40)
    $g.FillEllipse((Brush '#f6c453'), 117, 13, 22, 22)
    $g.FillEllipse((Brush '#fff6d6'), 122, 17, 8, 8)

    # Base: cream metal with a dark panel, rivets and indicator lights.
    $base = RoundRect 40 160 176 64 16
    $metal = New-Object System.Drawing.Drawing2D.LinearGradientBrush((Pt 0 160), (Pt 0 224), (C '#fbf3e1'), (C '#c9b994'))
    $g.FillPath((Brush '#0a2826' 90), (RoundRect 44 168 176 64 16))
    $g.FillPath($metal, $base)
    $g.DrawPath((RoundPen '#5b4f37' 5), $base)
    $g.FillPath((Brush '#1d2b2a'), (RoundRect 66 178 124 28 9))
    $lights = '#f6c453', '#f2878f', '#7fe0cf'
    for ($i = 0; $i -lt 3; $i++) {
        $x = 92 + $i * 36
        $g.FillEllipse((Brush $lights[$i] 80), $x - 12, 180, 24, 24)
        $g.FillEllipse((Brush $lights[$i]), $x - 7, 185, 14, 14)
    }
    if ($detail) { foreach ($rx in 52, 204) { $g.FillEllipse((Brush '#8a7a58'), $rx - 4, 188, 8, 8) } }

    # Glass dome with the brain inside.
    $dome = New-Object System.Drawing.Drawing2D.GraphicsPath
    $dome.AddArc(50, 50, 156, 220, 180, 180)
    $dome.CloseFigure()
    $g.FillPath((Brush '#bff3ea' 45), $dome)
    $g.SetClip($dome)
    DrawBrain $g 126 104 ($(if ($detail) { 0.92 } else { 1.0 })) $detail
    $g.ResetClip()
    $g.DrawPath((RoundPen '#f4ead5' ($(if ($detail) { 7 } else { 10 }))), $dome)
    # Glass highlight.
    $g.DrawArc((RoundPen '#ffffff' ($(if ($detail) { 9 } else { 12 })) 150), 66, 66, 124, 188, 200, 50)
    # Collar where the dome meets the base.
    $collar = RoundRect 44 150 168 20 8
    $g.FillPath((Brush '#e7d9b8'), $collar)
    $g.DrawPath((RoundPen '#5b4f37' 5), $collar)
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
$fs = [System.IO.File]::Create([IO.Path]::GetFullPath($Out))
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
