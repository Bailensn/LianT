package main

import (
	"image/color"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type C = layout.Context
type D = layout.Dimensions

func main() {
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Gio Demo"),
			app.Size(
				500,
				300,
			),
		)
		ui := newUI()
		var ops op.Ops
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				return
			case app.FrameEvent:
				gtx := app.NewContext(
					&ops,
					e,
				)
				ui.Layout(gtx)
				e.Frame(gtx.Ops)
			}
		}
	}()
	app.Main()
}

type UI struct {
	button widget.Clickable
	count int
	theme *material.Theme
}

func newUI() *UI {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(
		text.WithCollection(
			gofont.Collection(),
		),
	)
	return &UI{
		theme: th,
	}
}

func (ui *UI) Layout(gtx C) D {
	for ui.button.Clicked(gtx) {
		ui.count++
	}
	return layout.Flex{
		Axis: layout.Vertical,
		Alignment: layout.Middle,
	}.Layout(
		gtx,
		layout.Rigid(
			func(gtx C) D {
				title := material.H3(
					ui.theme,
					"Gio GUI Example",
				)
				title.Color = color.NRGBA{
					R:255,
					G:255,
					B:255,
					A:255,
				}
				return title.Layout(gtx)
			},
		),
		layout.Rigid(
			layout.Spacer{
				Height:20,
			}.Layout,
		),
		layout.Rigid(
			func(gtx C) D {
				btn := material.Button(
					ui.theme,
					&ui.button,
					"Click",
				)
				return btn.Layout(gtx)
			},
		),
		layout.Rigid(
			func(gtx C) D {
				txt := material.Body1(
					ui.theme,
					"Clicked: "+itoa(ui.count),
				)
				return txt.Layout(gtx)
			},
		),
	)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := make([]byte,0,10)
	for i>0 {
		buf = append(
			buf,
			byte('0'+i%10),
		)
		i/=10
	}
	for i,j:=0,len(buf)-1;i<j;i,j=i+1,j-1 {
		buf[i],buf[j]=buf[j],buf[i]
	}
	return string(buf)
}