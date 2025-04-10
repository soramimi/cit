package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// コミット情報を格納する構造体
type Commit struct {
	Hash          string
	Author        string
	Date          string
	Message       string
	IsUncommitted bool   // 未コミットの変更を表すフラグ
	Branch        string // コミットが属するブランチ名
	BranchLoaded  bool   // ブランチ情報が読み込まれたかどうか
	IsHead        bool   // HEADを指しているかどうか
}

// ブランチ情報のキャッシュ用マップとミューテックス
var (
	branchCache     = make(map[string]string) // コミットハッシュ -> ブランチ名のマッピング
	branchCacheLock sync.RWMutex
)

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

// 未コミットの変更があるか確認
func hasUncommittedChanges() bool {
	// git status --porcelain で未コミットの変更を確認
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()

	// エラーまたは出力が空の場合は未コミットの変更なし
	if err != nil || len(output) == 0 {
		return false
	}

	return true
}

// 未コミットの変更の概要を取得
func getUncommittedChangesSummary() (string, error) {
	// 変更されたファイルの数を取得
	cmdStatus := exec.Command("git", "status", "--porcelain")
	statusOutput, err := cmdStatus.Output()
	if err != nil {
		return "", err
	}

	// 行ごとに分割して数をカウント
	changedFiles := strings.Split(strings.TrimSpace(string(statusOutput)), "\n")
	numChanges := len(changedFiles)
	if numChanges == 0 {
		return "", nil // 変更なし
	}

	return fmt.Sprintf("%d files changed", numChanges), nil
}

// 現在のHEADのコミットハッシュを取得
func getHeadCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// ブランチのコミットハッシュを取得
func getBranchCommitHash(branchName string) (string, error) {
	cmd := exec.Command("git", "rev-parse", branchName)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// コミットが属するブランチを取得（キャッシュを活用）
func getCommitBranch(hash string) string {
	// キャッシュを確認
	branchCacheLock.RLock()
	branch, exists := branchCache[hash]
	branchCacheLock.RUnlock()

	if exists {
		return branch
	}

	// キャッシュになければ取得して保存
	cmd := exec.Command("git", "branch", "--contains", hash)
	output, err := cmd.Output()
	if err != nil {
		return "" // エラーの場合は空文字列を返す
	}

	// 出力から現在のブランチを探す
	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	foundBranch := ""
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if strings.HasPrefix(branch, "*") {
			// 現在のブランチの場合、「* 」を除去
			foundBranch = strings.TrimSpace(strings.TrimPrefix(branch, "*"))
			break
		} else if branch != "" && foundBranch == "" {
			// 最初に見つけたブランチを保存
			foundBranch = branch
		}
	}

	// キャッシュに保存
	branchCacheLock.Lock()
	branchCache[hash] = foundBranch
	branchCacheLock.Unlock()

	return foundBranch
}

// 一括でブランチマッピングを取得（高速化のため）
func getBranchesForCommits(commits []Commit) {
	// 非同期でマッピング情報を取得
	go func() {
		// すべてのブランチを一度だけ取得
		cmd := exec.Command("git", "branch", "-a", "--format=%(objectname) %(refname:short)")
		output, err := cmd.Output()
		if err != nil {
			return
		}

		// 結果をパースしてキャッシュに格納
		branchMappings := strings.Split(strings.TrimSpace(string(output)), "\n")
		branchCacheLock.Lock()
		for _, mapping := range branchMappings {
			parts := strings.SplitN(mapping, " ", 2)
			if len(parts) == 2 {
				commitHash := parts[0]
				branchName := parts[1]
				branchCache[commitHash] = branchName
			}
		}
		branchCacheLock.Unlock()
	}()
}

// 特定のコミットのブランチ情報を非同期で取得
func loadBranchInfoAsync(commit *Commit) {
	if commit.BranchLoaded || commit.IsUncommitted {
		return
	}

	go func(c *Commit) {
		// キャッシュをチェック
		branchCacheLock.RLock()
		branch, exists := branchCache[c.Hash]
		branchCacheLock.RUnlock()

		if exists {
			c.Branch = branch
			c.BranchLoaded = true
			return
		}

		// キャッシュになければ取得
		branch = getCommitBranch(c.Hash)
		c.Branch = branch
		c.BranchLoaded = true
	}(commit)
}

// コミットをチェックアウトする - switchとcheckoutを適切に使い分ける
func checkoutCommit(commit Commit) (string, error) {
	// ブランチ名が存在する場合、ブランチのHEADとコミットハッシュを比較
	if commit.Branch != "" {
		branchHash, err := getBranchCommitHash(commit.Branch)
		if err == nil && branchHash == commit.Hash {
			// ブランチのHEADとコミットハッシュが一致する場合はswitchを使用
			cmd := exec.Command("git", "switch", commit.Branch)
			output, err := cmd.CombinedOutput()
			return string(output), err
		}
	}

	// ブランチが存在しない場合、または一致しない場合はcheckoutでハッシュを指定
	cmd := exec.Command("git", "checkout", commit.Hash)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Gitコミットログを取得
func getGitCommits() ([]Commit, error) {
	// 現在のHEADのハッシュを取得
	headHash, err := getHeadCommitHash()
	if err != nil {
		// エラーがあってもプロセスは続行（HEADのハイライトができないだけ）
		headHash = ""
	}

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
			hash := parts[0]
			commit := Commit{
				Hash:          hash,
				Author:        parts[1],
				Date:          formatDate(parts[2]),
				Message:       formatMessage(parts[3]),
				IsUncommitted: false,
				BranchLoaded:  false,            // 初期状態では未ロード
				IsHead:        hash == headHash, // HEADかどうかをチェック
			}

			commits = append(commits, commit)
		}
	}

	// 未コミットの変更がある場合、先頭に追加
	if hasUncommittedChanges() {
		// 現在のユーザー名を取得
		userCmd := exec.Command("git", "config", "user.name")
		userName, _ := userCmd.Output()

		// 変更の概要を取得
		changesSummary, err := getUncommittedChangesSummary()
		if err != nil {
			changesSummary = "uncommitted changes"
		}

		// 現在の日時
		now := time.Now().Format("2006-01-02 15:04:05")

		// 未コミット変更を表すダミーコミットを作成
		uncommitted := Commit{
			Hash:          "--------",
			Author:        strings.TrimSpace(string(userName)),
			Date:          now,
			Message:       "Uncommitted Changes: " + changesSummary,
			IsUncommitted: true,
		}

		// 先頭に追加
		commits = append([]Commit{uncommitted}, commits...)
	}

	// 起動時の処理負荷を減らすため、ブランチ情報は後で非同期に読み込む
	getBranchesForCommits(commits)

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
		SetScrollable(true)

	// ステータス表示用の領域
	statusArea := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	// レイアウト設定 - FlexでTextViewの下に2行の余白を作成
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true).   // テキストビューが伸縮するように比率を設定
		AddItem(statusArea, 2, 0, false) // 下部に高さ2行の固定領域

	// 現在選択されているコミットのインデックス
	currentCommit := 0

	// スクロール位置の管理（先頭からの行オフセット）
	scrollOffset := 0

	// 確認モードのフラグ
	confirmMode := false

	// コミットを表示する関数
	displayCommits := func() {
		textView.Clear()

		// 画面幅と高さを取得（ステータス領域の分を考慮）
		_, _, width, height := textView.GetInnerRect()

		// 現在の選択位置が画面に表示されていないときのみスクロール位置を調整
		if currentCommit < scrollOffset {
			// 選択位置が画面の上方向に外れている場合、スクロール位置を選択位置に合わせる
			scrollOffset = currentCommit
		} else if currentCommit >= scrollOffset+height {
			// 選択位置が画面の下方向に外れている場合、スクロール位置を調整
			scrollOffset = currentCommit - height + 1
		}

		// 表示範囲内のコミットのブランチ情報を非同期でロード（表示されているものだけ）
		visibleStart := scrollOffset
		visibleEnd := scrollOffset + height
		if visibleEnd > len(commits) {
			visibleEnd = len(commits)
		}

		// 現在選択されているコミットの情報を優先的にロード
		if currentCommit >= 0 && currentCommit < len(commits) && !commits[currentCommit].IsUncommitted {
			loadBranchInfoAsync(&commits[currentCommit])
		}

		// 表示範囲内のコミットのブランチ情報を非同期でロード
		for i := visibleStart; i < visibleEnd; i++ {
			if i >= 0 && i < len(commits) && !commits[i].IsUncommitted {
				loadBranchInfoAsync(&commits[i])
			}
		}

		for i, commit := range commits {
			// 表示範囲内だけ処理
			if i < scrollOffset || i >= scrollOffset+height {
				continue
			}

			// 表示形式を変更: ハッシュ - 日付 - 作者 - メッセージ
			display := fmt.Sprintf("%s - %s - %s - %s", commit.Hash[:7], commit.Date, commit.Author, commit.Message)
			if commit.Branch != "" {
				display += fmt.Sprintf(" [%s]", commit.Branch)
			}

			// 画面幅に合わせて文字列を切り捨て
			if len(display) > width {
				display = display[:width]
			}

			// 表示スタイルの適用
			if i == currentCommit {
				// 現在選択されている行
				if commit.IsUncommitted {
					// 未コミットの変更を選択中の場合は特別な表示
					fmt.Fprintf(textView, "[black:yellow]%s[-:-]\n", display)
				} else {
					fmt.Fprintf(textView, "[black:white]%s[-:-]\n", display)
				}
			} else if commit.IsHead {
				// HEADを指しているコミットは黄色で表示
				fmt.Fprintf(textView, "[yellow]%s[-:-]\n", display)
			} else if commit.IsUncommitted {
				// 未コミットの変更は強調表示
				fmt.Fprintf(textView, "[yellow]%s[-:-]\n", display)
			} else {
				// 通常のコミット
				fmt.Fprintf(textView, "%s\n", display)
			}
		}

		// 計算済みのスクロール位置に直接移動
		textView.ScrollTo(scrollOffset, 0)

		// ステータスエリアの更新
		statusArea.Clear()
		if confirmMode && !commits[currentCommit].IsUncommitted {
			// 確認モード時: コミットチェックアウト確認メッセージを表示
			commit := commits[currentCommit]

			// チェックアウトするときのモード（ブランチswitchかdetached headか）を判定
			isDetachedHead := true
			branchToSwitch := ""
			if commit.Branch != "" {
				// ブランチのHEADとコミットハッシュを比較
				branchHash, err := getBranchCommitHash(commit.Branch)
				if err == nil && branchHash == commit.Hash {
					// ブランチのHEADとコミットハッシュが一致する場合
					isDetachedHead = false
					branchToSwitch = commit.Branch
				}
			}

			// メッセージ作成
			var checkoutMsg string
			if isDetachedHead {
				// detached headになる場合
				checkoutMsg = fmt.Sprintf("Checkout commit %s? (detached HEAD) [y/n]", commit.Hash[:7])
			} else {
				// ブランチにswitchする場合
				checkoutMsg = fmt.Sprintf("Checkout branch '%s'? [y/n]", branchToSwitch)
			}

			statusArea.Write([]byte(checkoutMsg))
		} else {
			// 通常時: コミット総数の表示
			branchInfo := ""
			if currentCommit < len(commits) && commits[currentCommit].Branch != "" {
				branchInfo = fmt.Sprintf(" (Branch: %s)", commits[currentCommit].Branch)
			}
			statusArea.Write([]byte(fmt.Sprintf("Total commits: %d%s", len(commits), branchInfo)))
		}
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
		// 確認モードの場合、y/n の入力を処理
		if confirmMode {
			switch event.Rune() {
			case 'y', 'Y':
				// 確認モードをオフにして戻る
				confirmMode = false
				commit := commits[currentCommit]

				// 未コミットの変更の場合は処理しない（安全策）
				if commit.IsUncommitted {
					displayCommits()
					return nil
				}

				// チェックアウトを実行
				output, err := checkoutCommit(commit)

				// ステータスエリアに結果を表示
				statusArea.Clear()
				if err != nil {
					statusArea.Write([]byte(fmt.Sprintf("Checkout failed: %v", err)))
				} else {
					// 成功時は短くメッセージを表示
					shortMsg := "Checkout successful"
					if len(output) > 0 {
						shortMsg = string(output)
						if len(shortMsg) > 60 { // 長すぎる場合は切り詰め
							shortMsg = shortMsg[:60] + "..."
						}
					}
					statusArea.Write([]byte(fmt.Sprintf("Checkout successful: %s", shortMsg)))
				}
				return nil

			case 'n', 'N':
				// キャンセルして通常モードに戻る
				confirmMode = false
				displayCommits()
				return nil
			}

			// その他のキーは無視
			return nil
		}

		// 通常モード時のキー処理
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

		case tcell.KeyEnter:
			// Enter: 確認モードに切り替え（ただし、uncommitted changesの場合は何もしない）
			if !commits[currentCommit].IsUncommitted {
				confirmMode = true
				displayCommits()
			}
			return nil
		}

		return event
	})

	// アプリケーション全体のキー入力のハンドリング
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			// 確認モード中のEscapeは確認モードを解除するだけ
			if confirmMode {
				confirmMode = false
				displayCommits()
				return nil
			}
			// 通常モード中のEscapeはアプリケーションを終了
			app.Stop()
			return nil
		}
		return event
	})

	// 画面の初期化と表示更新を確実に行うためのセットアップ
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		// 画面をクリア
		screen.Clear()
		return false // 通常の描画処理を継続
	})

	// 定期的に画面更新とHEADの位置更新を行うタイマー
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		for range ticker.C {
			app.QueueUpdateDraw(func() {
				// 最新のHEADの位置を取得
				headHash, err := getHeadCommitHash()
				if err == nil {
					// HEADの位置を更新
					for i := range commits {
						if !commits[i].IsUncommitted {
							commits[i].IsHead = (commits[i].Hash == headHash)
						}
					}
				}

				// 画面を更新
				if currentCommit >= 0 && currentCommit < len(commits) {
					displayCommits()
				}
			})
		}
	}()

	// アプリケーション実行
	// QueueUpdateDrawを最初に一度だけ使用するように修正
	go func() {
		// アプリケーションの起動を少し待機
		time.Sleep(100 * time.Millisecond)

		// 一回だけ安全に再描画を行う
		app.QueueUpdateDraw(func() {
			displayCommits() // コミットの再表示
		})
	}()

	// メインレイアウト（flex）をルートとして設定
	if err := app.SetRoot(flex, true).Run(); err != nil {
		panic(err)
	}
}
