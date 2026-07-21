[CmdletBinding()]
param(
    [string]$OutputPath = (Join-Path $PSScriptRoot '..\images\ai-cli-banner.png')
)

Add-Type -AssemblyName System.Drawing

function New-Color {
    param(
        [string]$Hex,
        [int]$Alpha = 255
    )

    $base = [System.Drawing.ColorTranslator]::FromHtml($Hex)
    return [System.Drawing.Color]::FromArgb($Alpha, $base.R, $base.G, $base.B)
}

function New-RoundedPath {
    param(
        [float]$X,
        [float]$Y,
        [float]$Width,
        [float]$Height,
        [float]$Radius
    )

    $path = New-Object System.Drawing.Drawing2D.GraphicsPath
    $diameter = [Math]::Min($Radius * 2, [Math]::Min($Width, $Height))
    $arc = New-Object System.Drawing.RectangleF($X, $Y, $diameter, $diameter)

    $path.AddArc($arc, 180, 90)
    $arc.X = $X + $Width - $diameter
    $path.AddArc($arc, 270, 90)
    $arc.Y = $Y + $Height - $diameter
    $path.AddArc($arc, 0, 90)
    $arc.X = $X
    $path.AddArc($arc, 90, 90)
    $path.CloseFigure()
    return $path
}

function New-FontSafe {
    param(
        [string[]]$Names,
        [float]$Size,
        [System.Drawing.FontStyle]$Style
    )

    foreach ($name in $Names) {
        try {
            $font = New-Object System.Drawing.Font($name, $Size, $Style, [System.Drawing.GraphicsUnit]::Pixel)
            if ($font.Name) {
                return $font
            }
        } catch {
        }
    }

    return New-Object System.Drawing.Font('Segoe UI', $Size, $Style, [System.Drawing.GraphicsUnit]::Pixel)
}

$OutputPath = [System.IO.Path]::GetFullPath($OutputPath)
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null

$width = 1600
$height = 900

$bitmap = New-Object System.Drawing.Bitmap($width, $height)
$graphics = [System.Drawing.Graphics]::FromImage($bitmap)

$graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
$graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
$graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
$graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
$graphics.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::ClearTypeGridFit

$backgroundRect = New-Object System.Drawing.RectangleF(0, 0, $width, $height)
$backgroundBrush = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
    $backgroundRect,
    (New-Color '#071a2f'),
    (New-Color '#102e4f'),
    28
)
$graphics.FillRectangle($backgroundBrush, 0, 0, $width, $height)

$topGlowPath = New-Object System.Drawing.Drawing2D.GraphicsPath
$topGlowPath.AddEllipse(700, -150, 860, 620)
$topGlowBrush = New-Object System.Drawing.Drawing2D.PathGradientBrush($topGlowPath)
$topGlowBrush.CenterColor = New-Color '#22d3ee' 110
$topGlowBrush.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 0, 0, 0))
$graphics.FillPath($topGlowBrush, $topGlowPath)

$warmGlowPath = New-Object System.Drawing.Drawing2D.GraphicsPath
$warmGlowPath.AddEllipse(980, 520, 520, 240)
$warmGlowBrush = New-Object System.Drawing.Drawing2D.PathGradientBrush($warmGlowPath)
$warmGlowBrush.CenterColor = New-Color '#f59e0b' 82
$warmGlowBrush.SurroundColors = @([System.Drawing.Color]::FromArgb(0, 0, 0, 0))
$graphics.FillPath($warmGlowBrush, $warmGlowPath)

$gridPen = New-Object System.Drawing.Pen((New-Color '#7dd3fc' 24), 1)
for ($x = -320; $x -lt ($width + 320); $x += 86) {
    $graphics.DrawLine($gridPen, $x, 0, $x - 180, $height)
}

for ($y = 66; $y -lt $height; $y += 92) {
    $rowPen = New-Object System.Drawing.Pen((New-Color '#bfdbfe' 12), 1)
    $graphics.DrawLine($rowPen, 0, $y, $width, $y)
    $rowPen.Dispose()
}

$dotBrush = New-Object System.Drawing.SolidBrush((New-Color '#dbeafe' 40))
$dotAccentBrush = New-Object System.Drawing.SolidBrush((New-Color '#67e8f9' 58))
$graphics.FillEllipse($dotBrush, 1168, 108, 10, 10)
$graphics.FillEllipse($dotBrush, 1202, 254, 6, 6)
$graphics.FillEllipse($dotBrush, 1446, 148, 9, 9)
$graphics.FillEllipse($dotAccentBrush, 1342, 300, 12, 12)

$terminalX = 92
$terminalY = 156
$terminalWidth = 654
$terminalHeight = 554

$terminalShadowPath = New-RoundedPath ($terminalX + 18) ($terminalY + 18) $terminalWidth $terminalHeight 34
$terminalShadowBrush = New-Object System.Drawing.SolidBrush((New-Color '#020617' 110))
$graphics.FillPath($terminalShadowBrush, $terminalShadowPath)

$terminalPath = New-RoundedPath $terminalX $terminalY $terminalWidth $terminalHeight 34
$terminalFillBrush = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
    (New-Object System.Drawing.RectangleF($terminalX, $terminalY, $terminalWidth, $terminalHeight)),
    (New-Color '#0b1220' 240),
    (New-Color '#111c30' 245),
    90
)
$terminalBorderPen = New-Object System.Drawing.Pen((New-Color '#7dd3fc' 70), 2)
$graphics.FillPath($terminalFillBrush, $terminalPath)
$graphics.DrawPath($terminalBorderPen, $terminalPath)

$terminalHeaderPath = New-RoundedPath $terminalX $terminalY $terminalWidth 84 34
$terminalHeaderBrush = New-Object System.Drawing.SolidBrush((New-Color '#111827' 228))
$terminalDividerPen = New-Object System.Drawing.Pen((New-Color '#334155' 180), 1)
$graphics.FillPath($terminalHeaderBrush, $terminalHeaderPath)
$graphics.DrawLine($terminalDividerPen, $terminalX, ($terminalY + 84), ($terminalX + $terminalWidth), ($terminalY + 84))

$windowRed = New-Object System.Drawing.SolidBrush((New-Color '#fb7185'))
$windowAmber = New-Object System.Drawing.SolidBrush((New-Color '#f59e0b'))
$windowGreen = New-Object System.Drawing.SolidBrush((New-Color '#34d399'))
$graphics.FillEllipse($windowRed, $terminalX + 20, $terminalY + 22, 16, 16)
$graphics.FillEllipse($windowAmber, $terminalX + 52, $terminalY + 22, 16, 16)
$graphics.FillEllipse($windowGreen, $terminalX + 84, $terminalY + 22, 16, 16)

$headerFont = New-FontSafe @('Segoe UI Semibold', 'Segoe UI') 23 ([System.Drawing.FontStyle]::Bold)
$monoPromptFont = New-FontSafe @('Cascadia Code', 'Consolas') 24 ([System.Drawing.FontStyle]::Regular)
$monoBodyFont = New-FontSafe @('Cascadia Code', 'Consolas') 18 ([System.Drawing.FontStyle]::Regular)
$titleFont = New-FontSafe @('Segoe UI Semibold', 'Segoe UI') 82 ([System.Drawing.FontStyle]::Bold)
$subtitleFont = New-FontSafe @('Segoe UI', 'Segoe UI Variable') 27 ([System.Drawing.FontStyle]::Regular)
$badgeFont = New-FontSafe @('Segoe UI Semibold', 'Segoe UI') 23 ([System.Drawing.FontStyle]::Bold)
$metaFont = New-FontSafe @('Segoe UI Semibold', 'Segoe UI') 19 ([System.Drawing.FontStyle]::Bold)
$providerFont = New-FontSafe @('Segoe UI', 'Segoe UI Variable') 18 ([System.Drawing.FontStyle]::Regular)
$chipFont = New-FontSafe @('Segoe UI Semibold', 'Segoe UI') 19 ([System.Drawing.FontStyle]::Bold)

$headerTextBrush = New-Object System.Drawing.SolidBrush((New-Color '#cbd5e1'))
$cyanBrush = New-Object System.Drawing.SolidBrush((New-Color '#67e8f9'))
$mutedBrush = New-Object System.Drawing.SolidBrush((New-Color '#b7c4da'))
$amberBrush = New-Object System.Drawing.SolidBrush((New-Color '#fbbf24'))
$greenBrush = New-Object System.Drawing.SolidBrush((New-Color '#86efac'))
$whiteBrush = New-Object System.Drawing.SolidBrush((New-Color '#f8fafc'))
$lightBlueBrush = New-Object System.Drawing.SolidBrush((New-Color '#dbeafe'))
$accentBrush = New-Object System.Drawing.SolidBrush((New-Color '#7dd3fc'))
$badgeTextBrush = New-Object System.Drawing.SolidBrush((New-Color '#ecfeff'))

$graphics.DrawString('session: natural language -> shell', $headerFont, $headerTextBrush, $terminalX + 116, $terminalY + 15)

$bodyX = $terminalX + 40
$promptY = $terminalY + 118
$promptRect = New-Object System.Drawing.RectangleF($bodyX, $promptY, 500, 72)
$promptFormat = New-Object System.Drawing.StringFormat
$graphics.DrawString("$ ai show what process`nis using port 8080", $monoPromptFont, $cyanBrush, $promptRect, $promptFormat)
$graphics.DrawString('model> interpret request and choose shell-safe lookup', $monoBodyFont, $mutedBrush, $bodyX, $promptY + 92)
$graphics.DrawString('safety> risky: no    certainty: 92', $monoBodyFont, $amberBrush, $bodyX, $promptY + 146)
$graphics.DrawString('action: auto-run', $monoBodyFont, $amberBrush, $bodyX, $promptY + 190)
$graphics.DrawString('run> lsof -i :8080', $monoBodyFont, $whiteBrush, $bodyX, $promptY + 250)
$graphics.DrawString('out> node 24871 LISTEN TCP *:8080', $monoBodyFont, $greenBrush, $bodyX, $promptY + 304)

$badgeX = $bodyX
$badgeY = $terminalY + 458
$badgePath = New-RoundedPath $badgeX $badgeY 286 60 18
$badgeFill = New-Object System.Drawing.SolidBrush((New-Color '#0f766e' 200))
$badgeBorderPen = New-Object System.Drawing.Pen((New-Color '#5eead4' 180), 2)
$graphics.FillPath($badgeFill, $badgePath)
$graphics.DrawPath($badgeBorderPen, $badgePath)
$graphics.DrawString('review before execute', $badgeFont, $badgeTextBrush, $badgeX + 22, $badgeY + 13)

$providerX = 1032
$providerY = 121
$providerWidth = 436
$providerHeight = 122
$providerPath = New-RoundedPath $providerX $providerY $providerWidth $providerHeight 28
$providerFill = New-Object System.Drawing.SolidBrush((New-Color '#0f172a' 150))
$providerBorderPen = New-Object System.Drawing.Pen((New-Color '#67e8f9' 60), 1.6)
$graphics.FillPath($providerFill, $providerPath)
$graphics.DrawPath($providerBorderPen, $providerPath)
$graphics.DrawString('providers', $metaFont, $accentBrush, $providerX + 28, $providerY + 4)
$graphics.DrawString('OpenAI | OpenRouter | Local', $providerFont, $whiteBrush, $providerX + 28, $providerY + 40)
$graphics.DrawString('safety policy gates every command', $providerFont, $lightBlueBrush, $providerX + 28, $providerY + 74)

$titleX = 900
$titleY = 302
$graphics.DrawString('AI CLI', $titleFont, $whiteBrush, $titleX, $titleY)

$underlinePen = New-Object System.Drawing.Pen((New-Color '#22d3ee' 230), 5)
$graphics.DrawLine($underlinePen, $titleX + 12, 422, $titleX + 168, 422)

$subtitleRect = New-Object System.Drawing.RectangleF($titleX, 444, 520, 166)
$subtitleFormat = New-Object System.Drawing.StringFormat
$graphics.DrawString(
    'Translate plain English into shell commands, then apply a safety policy before execution.',
    $subtitleFont,
    $lightBlueBrush,
    $subtitleRect,
    $subtitleFormat
)

$chipY = 696
$chipFill = New-Object System.Drawing.SolidBrush((New-Color '#082032' 176))
$chipBorderPen = New-Object System.Drawing.Pen((New-Color '#38bdf8' 80), 1.5)

$chip1 = New-RoundedPath 888 $chipY 190 56 16
$chip2 = New-RoundedPath 1096 $chipY 196 56 16
$chip3 = New-RoundedPath 1310 $chipY 168 56 16

$graphics.FillPath($chipFill, $chip1)
$graphics.FillPath($chipFill, $chip2)
$graphics.FillPath($chipFill, $chip3)
$graphics.DrawPath($chipBorderPen, $chip1)
$graphics.DrawPath($chipBorderPen, $chip2)
$graphics.DrawPath($chipBorderPen, $chip3)

$graphics.DrawString('safety aware', $chipFont, $badgeTextBrush, 920, $chipY + 14)
$graphics.DrawString('multi-provider', $chipFont, $badgeTextBrush, 1126, $chipY + 14)
$graphics.DrawString('cross-shell', $chipFont, $badgeTextBrush, 1342, $chipY + 14)

$bitmap.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Png)

$chip3.Dispose()
$chip2.Dispose()
$chip1.Dispose()
$chipBorderPen.Dispose()
$chipFill.Dispose()
$subtitleFormat.Dispose()
$underlinePen.Dispose()
$providerBorderPen.Dispose()
$providerFill.Dispose()
$providerPath.Dispose()
$badgeBorderPen.Dispose()
$badgeFill.Dispose()
$badgePath.Dispose()
$promptFormat.Dispose()
$badgeTextBrush.Dispose()
$accentBrush.Dispose()
$lightBlueBrush.Dispose()
$whiteBrush.Dispose()
$greenBrush.Dispose()
$amberBrush.Dispose()
$mutedBrush.Dispose()
$cyanBrush.Dispose()
$headerTextBrush.Dispose()
$chipFont.Dispose()
$providerFont.Dispose()
$metaFont.Dispose()
$badgeFont.Dispose()
$subtitleFont.Dispose()
$titleFont.Dispose()
$monoBodyFont.Dispose()
$monoPromptFont.Dispose()
$headerFont.Dispose()
$windowGreen.Dispose()
$windowAmber.Dispose()
$windowRed.Dispose()
$terminalDividerPen.Dispose()
$terminalHeaderBrush.Dispose()
$terminalHeaderPath.Dispose()
$terminalBorderPen.Dispose()
$terminalFillBrush.Dispose()
$terminalPath.Dispose()
$terminalShadowBrush.Dispose()
$terminalShadowPath.Dispose()
$dotAccentBrush.Dispose()
$dotBrush.Dispose()
$gridPen.Dispose()
$warmGlowBrush.Dispose()
$warmGlowPath.Dispose()
$topGlowBrush.Dispose()
$topGlowPath.Dispose()
$backgroundBrush.Dispose()
$graphics.Dispose()
$bitmap.Dispose()

Get-Item $OutputPath | Select-Object FullName, Length