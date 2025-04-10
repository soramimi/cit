package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()
	
	// テキスト表示コンポーネント
	textView := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText("Hello, world")
	
	// レイアウト設定
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(textView, 0, 1, true).
			AddItem(nil, 0, 1, false),
		0, 1, true).
		AddItem(nil, 0, 1, false)
	
	// Escキーでアプリケーション終了
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
			return nil
		}
		return event
	})
	
	// アプリケーション実行
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
