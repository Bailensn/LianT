# Project: LianT (app module)

- Package `com.lensnteam.liant`, launcher = `SplashActivity` (ComponentActivity). namespace `com.lensnteam.liant`, minSdk 28, compileSdk 37, targetSdk 36, manifest theme `Theme.App`, icon `@drawable/ic_launcher` (ic_launcher.xml).
- Deps: material3 **1.5.0-alpha26** (M3 Expressive: `MaterialExpressiveTheme`, `expressiveLightColorScheme`, `@OptIn(ExperimentalMaterial3ExpressiveApi::class)`), **io.github.kyant0:backdrop:2.0.0**, material-icons-extended 1.7.8, ui 1.12.0-rc01 / foundation 1.13.0-alpha01, navigation-compose.
- IMPORTANT compiler quirk: `material-icons` extension properties (`Icons.Filled.ChevronRight`, `Icons.AutoMirrored.Filled.KeyboardArrowRight`) do NOT resolve on this CodeAssist compiler (verified with a probe). Use custom Canvas/path-drawn icons instead.
- Build via `bundle:app:debug` task; verified passing after SplashActivity rewrite.

## Backdrop 2.0.0 verified API (from GitHub tag 2.0.0 sources + gitbook docs)

- `Modifier.drawBackdrop(backdrop: Backdrop, shape: () -> Shape, effects: BackdropEffectScope.() -> Unit, highlight: (() -> Highlight?)?, shadow: (() -> Shadow?)?, innerShadow: (() -> InnerShadow?)?, layerBlock, exportedBackdrop: LayerBackdrop?, onDrawBehind, onDrawBackdrop, onDrawSurface, onDrawFront)`.
- `rememberLayerBackdrop(): LayerBackdrop` + `Modifier.layerBackdrop(backdrop: LayerBackdrop)` (records the composable content). `exportedBackdrop` derives a LayerBackdrop from a drawBackdrop for **glass-on-glass** (docs: never nest layerBackdrop inside drawBackdrop content → SIGSEGV).
- Effects order MUST be **color filter ⇒ blur ⇒ lens**: `vibrancy(); blur(radiusPx); lens(refractionHeight, refractionAmount, depthEffect, chromaticAberration)`. `effects` lambda is a Density so `dp.toPx()` works there.
- `Highlight(width: Dp = 0.5.dp, blurRadius: Dp = width/2, alpha: Float, style: HighlightStyle)` — Dp params, NOT px; the old draft's `Highlight(radius=..., alpha=...)` was a compile error.
- `HighlightStyle.Default` is ambiguous (nested classifier shadows companion val) → write `HighlightStyle.Default()`.
- `Shadow(radius: Dp, offset: DpOffset, color: Color, alpha: Float, blendMode)`; `InnerShadow(radius: Dp, offset: DpOffset, color: Color, alpha, blendMode)`.
- `lens` needs CornerBasedShape (RoundedCornerShape OK) + Android 13+ (RuntimeShader); blur works 12+; `vibrancy()` = saturation ×1.5.
- SplashActivity.kt structure: edge-to-edge (SystemBarStyle.auto transparent + isNavigationBarContrastEnforced=false on Q+), LianTTheme (dynamic light/dark on 12+, fallbacks below), BotSelectScreen = root Box{ flat bg + LiquidBackground(layerBackdrop) + foreground Column{ AppLogo, title/subtitle, BotGlassPanel(drawBackdrop from backgroundBackdrop, exportedBackdrop=panelBackdrop, items drawBackdrop from panelBackdrop), AddAccountText(Text + TextButton only) } }, Light/Dark previews.
