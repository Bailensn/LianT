package main

import (
	"image"
	"image/color"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type C = layout.Context
type D = layout.Dimensions


func roundedRect(
	gtx C,
	c color.NRGBA,
	radius int,
) {
	rect := image.Rectangle{
		Max: gtx.Constraints.Max,
	}

	rr := clip.RRect{
		Rect: rect,
		NE: radius,
		NW: radius,
		SE: radius,
		SW: radius,
	}

	paint.FillShape(
		gtx.Ops,
		c,
		rr.Op(gtx.Ops),
	)
}



func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("联T"),
			app.Size(
				unit.Dp(300),
				unit.Dp(200),
			),
		)
		var ops op.Ops
		var quit widget.Clickable
		theme := material.NewTheme()
		for {
			e := w.Event()
			switch e := e.(type) {
			case app.FrameEvent:
				gtx := app.NewContext(
					&ops,
					e,
				)
				roundedRect(
					gtx,
					color.NRGBA{
						R:50,
						G:120,
						B:220,
						A:255,
					},
					25,
				)
				layout.Inset{
					Top: unit.Dp(20),
					Left: unit.Dp(20),
				}.Layout(
					gtx,
					func(gtx C) D {
						return layout.Flex{
							Axis: layout.Vertical,
						}.Layout(
							gtx,
							layout.Rigid(
								func(gtx C) D {
									return material.H6(
										theme,
										"联T",
									).Layout(gtx)
								},
							),
							layout.Rigid(
								func(gtx C) D {
									return layout.Spacer{
										Height: unit.Dp(80),
									}.Layout(gtx)
								},
							),
							layout.Rigid(
								func(gtx C) D {
									btn :=
										material.Button(
											theme,
											&quit,
											"退出",
										)
									if quit.Clicked(gtx) {
										w.Perform(
											system.ActionClose,
										)
									}
									return btn.Layout(gtx)
								},
							),
						)
					},
				)
				e.Frame(
					gtx.Ops,
				)
			case app.DestroyEvent:
				return
			}
		}
	}()
	app.Main()
}