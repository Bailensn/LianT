// Package ui provides the Gio-based startup (splash) window shown by the
// launcher while it provisions the Python runtime and boots the client,
// similar to the splash screens of Android Studio or WPS Office.
package ui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"sync/atomic"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget/material"

	// embed is required by the //go:embed directive below.
	_ "embed"

	// Register the built-in Go font so material labels render without extra files.
	_ "gioui.org/font/gofont"
)

//go:embed logo.png
var logoPNG []byte

// Palette used by the splash.
var (
	bgDark     = color.NRGBA{R: 0x16, G: 0x1B, B: 0x26, A: 0xFF} // deep navy
	accent     = color.NRGBA{R: 0x4F, G: 0x8B, B: 0xFF, A: 0xFF} // brand blue
	textLight  = color.NRGBA{R: 0xF5, G: 0xF7, B: 0xFC, A: 0xFF}
	textMuted  = color.NRGBA{R: 0x9F, G: 0xA8, B: 0xB8, A: 0xFF}
	progressBg = color.NRGBA{R: 0x2A, G: 0x32, B: 0x42, A: 0xFF}
)

// Splash is a small, frameless startup window with a brand mark, a status
// message and an indeterminate progress bar.
type Splash struct {
	win     *app.Window
	theme   *material.Theme
	logo    image.Image
	status  atomic.Value // string
	closed  atomic.Bool
	done    chan struct{}
	startAt time.Time
}

// NewSplash creates a splash window for the given version string.
func NewSplash(version string) *Splash {
	s := &Splash{
		theme:   material.NewTheme(),
		done:    make(chan struct{}),
		startAt: time.Now(),
	}
	if decoded, err := png.Decode(bytes.NewReader(logoPNG)); err == nil {
		s.logo = decoded
	}
	s.status.Store("Preparing")
	s.win = new(app.Window)
	s.win.Option(
		app.Title("LianT "+version),
		app.Size(unit.Dp(420), unit.Dp(240)),
		app.MinSize(unit.Dp(360), unit.Dp(200)),
		app.MaxSize(unit.Dp(560), unit.Dp(360)),
		app.Decorated(false),
	)
	return s
}

// SetStatus updates the status text shown on the splash. It is safe to call
// from any goroutine.
func (s *Splash) SetStatus(msg string) {
	s.status.Store(msg)
	if s.win != nil {
		s.win.Invalidate()
	}
}

// Close requests the splash window to close. Safe to call from any goroutine.
func (s *Splash) Close() {
	if s.closed.Swap(true) {
		return
	}
	close(s.done)
	if s.win != nil {
		s.win.Perform(system.ActionClose)
	}
}

// Run blocks while the splash window is displayed, returning once it closes.
// Run must be called in its own goroutine; the launcher's main goroutine
// drives the window event loop via app.Main.
func (s *Splash) Run() {
	var ops op.Ops
	for {
		select {
		case <-s.done:
			s.win.Perform(system.ActionClose)
			return
		default:
		}
		switch e := s.win.Event().(type) {
		case app.DestroyEvent:
			return
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			s.layout(gtx)
			// Request the next frame so the indeterminate bar keeps animating.
			gtx.Execute(op.InvalidateCmd{At: e.Now.Add(time.Second / 30)})
			e.Frame(gtx.Ops)
		}
	}
}

// layout renders a full-window background, a centered brand block and the
// status footer.
func (s *Splash) layout(gtx layout.Context) layout.Dimensions {
	paint.Fill(gtx.Ops, bgDark)
	return layout.Flex{
		Axis:      layout.Vertical,
		Alignment: layout.Middle,
		Spacing:   layout.SpaceEvenly,
	}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.drawLogo(gtx)
		}),
		layout.Rigid(s.drawName),
		layout.Rigid(s.drawProgress),
		layout.Rigid(s.drawStatus),
	)
}

// drawLogo paints the embedded brand mark at a fixed Dp size. Falls back to a
// plain accent tile if the image failed to decode.
func (s *Splash) drawLogo(gtx layout.Context) layout.Dimensions {
	size := gtx.Dp(unit.Dp(96))
	if s.logo != nil {
		imgOp := paint.NewImageOp(s.logo)
		imgOp.Add(gtx.Ops)
		b := s.logo.Bounds()
		sx := float32(size) / float32(b.Dx())
		sy := float32(size) / float32(b.Dy())
		defer op.Affine(f32.Affine2D{}.Scale(f32.Pt(0, 0), f32.Pt(sx, sy))).Push(gtx.Ops).Pop()
		paint.PaintOp{}.Add(gtx.Ops)
		return layout.Dimensions{Size: image.Pt(size, size)}
	}

	// Fallback: a rounded accent tile with a white bolt.
	rect := image.Rect(0, 0, size, size)
	rad := gtx.Dp(unit.Dp(20))
	defer clip.RRect{Rect: rect, SE: rad, SW: rad, NW: rad, NE: rad}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, accent)
	boltW := gtx.Dp(unit.Dp(16))
	boltH := gtx.Dp(unit.Dp(32))
	deg := float64(24) * math.Pi / 180
	center := f32.Pt(float32(size)/2, float32(size)/2)
	defer op.Affine(f32.Affine2D{}.Rotate(center, float32(deg))).Push(gtx.Ops).Pop()
	boltRect := image.Rect(
		size/2-boltW/2, size/2-boltH/2,
		size/2+boltW/2, size/2+boltH/2,
	)
	paint.FillShape(gtx.Ops, textLight, clip.RRect{Rect: boltRect}.Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// drawName renders the product name and a short hint.
func (s *Splash) drawName(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			name := material.Label(s.theme, unit.Sp(30), "LianT")
			name.Font.Weight = font.Bold
			name.Color = textLight
			return name.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			hint := material.Label(s.theme, unit.Sp(12), "A fast, cross-platform IM client")
			hint.Color = textMuted
			return hint.Layout(gtx)
		}),
	)
}

// drawProgress renders an indeterminate animated bar.
func (s *Splash) drawProgress(gtx layout.Context) layout.Dimensions {
	barW := gtx.Dp(unit.Dp(220))
	barH := gtx.Dp(unit.Dp(6))
	rad := barH / 2

	barRect := image.Rect(0, 0, barW, barH)
	rtrack := clip.RRect{Rect: barRect, SE: rad, SW: rad, NW: rad, NE: rad}.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, progressBg, rtrack)

	// Animated segment sweeping across the track.
	elapsed := time.Since(s.startAt).Seconds()
	phase := float32(math.Mod(elapsed*0.6, 1.0)) // 0..1
	segW := barW * 3 / 10
	pos := int((phase-0.15)*float32(barW)+float32(segW)) - segW
	if pos < 0 {
		pos = 0
	}
	if pos+segW > barW {
		pos = barW - segW
	}
	segRect := image.Rect(pos, 0, pos+segW, barH)
	paint.FillShape(gtx.Ops, accent, clip.RRect{Rect: segRect, SE: rad, SW: rad, NW: rad, NE: rad}.Op(gtx.Ops))

	return layout.Dimensions{Size: image.Pt(barW, barH)}
}

// drawStatus renders the current status message.
func (s *Splash) drawStatus(gtx layout.Context) layout.Dimensions {
	msg, _ := s.status.Load().(string)
	lbl := material.Label(s.theme, unit.Sp(13), msg)
	lbl.Color = textMuted
	return layout.Inset{Bottom: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return lbl.Layout(gtx)
	})
}
