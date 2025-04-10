package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// コミット情報を格納する構造体
type Commit struct {
	Hash    string
	Author  string
	Date    string
	Message string
}

// Gitリポジトリが存在するか確認
func checkGitRepository() bool {
	_, err := os.Stat(".git")
	return err == nil
}

// 日時文字列をyyyy-MM-dd HH:mm:ss形式に変換
func formatDate(dateStr string) string {
	// Gitが返す標準形式の日時文字列を解析
	t, err := time.Parse("Mon Jan 2 15:04:05 2006 -0700", dateStr)
	if err != nil {
		// パース失敗した場合は元の文字列を返す
		return dateStr
	}

	// 指定された形式に変換して返す
	return t.Format("2006-01-02 15:04:05")
}

// コミットメッセージの改行をスペースに置換
func formatMessage(message string) string {
	return strings.ReplaceAll(message, "\n", " ")
}

// Gitコミットログを取得
func getGitCommits() ([]Commit, error) {
	// 日時をGitの標準形式で取得
	cmd := exec.Command("git", "log", "--all", "--pretty=format:%H|%an|%ad|%s")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			commits = append(commits, Commit{
				Hash:    parts[0],
				Author:  parts[1],
				Date:    formatDate(parts[2]),
				Message: formatMessage(parts[3]),
			})
		}
	}
	return commits, nil
}

func main() {
	// Gitリポジトリの存在確認
	if !checkGitRepository() {
		fmt.Println("エラー: カレントディレクトリにGitリポジトリが存在しません。")
		os.Exit(1)
	}

	// Gitコミットログを取得
	commits, err := getGitCommits()
	if err != nil {
		fmt.Printf("エラー: Gitコミットログの取得に失敗しました: %v\n", err)
		os.Exit(1)
	}

	app := tview.NewApplication()

	// コミットログ表示用のTextViewを使用して、より細かい制御を可能にする
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	// 現在選択されているコミットのインデックス
	currentCommit := 0

	// スクロール位置の管理（先頭からの行オフセット）
	scrollOffset := 0

	// コミットを表示する関数
	displayCommits := func() {
		textView.Clear()

		// 画面幅と高さを取得
		_, _, width, height := textView.GetInnerRect()

		// 現在の選択位置が画面に表示されていないときのみスクロール位置を調整
		if currentCommit < scrollOffset {
			// 選択位置が画面の上方向に外れている場合、スクロール位置を選択位置に合わせる
			scrollOffset = currentCommit
		} else if currentCommit >= scrollOffset+height {
			// 選択位置が画面の下方向に外れている場合、スクロール位置を調整
			scrollOffset = currentCommit - height + 1
		}

		for i, commit := range commits {
			// 表示形式を変更: ハッシュ - 日付 - 作者 - メッセージ
			display := fmt.Sprintf("%s - %s - %s - %s", commit.Hash[:7], commit.Date, commit.Author, commit.Message)

			// 画面幅に合わせて文字列を切り捨て
			if len(display) > width {
				display = display[:width]
			}

			// 選択中のコミットは反転表示、それ以外は通常表示
			if i == currentCommit {
				fmt.Fprintf(textView, "[black:white]%s[-:-]\n", display)
			} else {
				fmt.Fprintf(textView, "%s\n", display)
			}
		}

		// 計算済みのスクロール位置に直接移動
		textView.ScrollTo(scrollOffset, 0)
	}

	// 初期表示
	if len(commits) > 0 {
		displayCommits()
	}

	// 1ページあたりの行数を計算
	getPageSize := func() int {
		_, _, _, height := textView.GetInnerRect()
		return height - 1 // 境界調整
	}

	// キー入力のハンドリング
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if currentCommit > 0 {
				currentCommit--
				displayCommits()
			}
			return nil

		case tcell.KeyDown:
			if currentCommit < len(commits)-1 {
				currentCommit++
				displayCommits()
			}
			return nil

		case tcell.KeyPgUp:
			// Page Up: 1ページ分上にスクロール
			pageSize := getPageSize()
			if currentCommit >= pageSize {
				currentCommit -= pageSize
			} else {
				currentCommit = 0 // 先頭へ
			}
			displayCommits()
			return nil

		case tcell.KeyPgDn:
			// Page Down: 1ページ分下にスクロール
			pageSize := getPageSize()
			if currentCommit+pageSize < len(commits) {
				currentCommit += pageSize
			} else {
				currentCommit = len(commits) - 1 // 最後尾へ
			}
			displayCommits()
			return nil
		}

		return event
	})

	// アプリケーション全体のキー入力のハンドリング
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			app.Stop()
			return nil
		}
		return event
	})

	// アプリケーション実行
	if err := app.SetRoot(textView, true).Run(); err != nil {
		panic(err)
	}
}
