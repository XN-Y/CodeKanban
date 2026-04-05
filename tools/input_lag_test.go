// 测试终端在快速输出时输入是否卡顿
// 运行: go run tools/input_lag_test.go
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	// 启动一个 goroutine 快速更新上方区域
	stopCh := make(chan struct{})
	inputCh := make(chan string)

	go func() {
		counter := 0
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

		for {
			select {
			case <-stopCh:
				return
			default:
				counter++
				s := spinner[counter%len(spinner)]

				// 移动到第1行，清除并打印
				// 使用 ANSI 转义序列
				fmt.Printf("\033[s")           // 保存光标位置
				fmt.Printf("\033[1;1H")        // 移动到第1行第1列
				fmt.Printf("\033[K")           // 清除当前行
				fmt.Printf("%s 快速更新计数器: %d | 时间: %s",
					s, counter, time.Now().Format("15:04:05.000"))
				fmt.Printf("\033[u")           // 恢复光标位置

				time.Sleep(10 * time.Millisecond) // 100 FPS 更新
			}
		}
	}()

	// 设置屏幕
	fmt.Print("\033[2J")     // 清屏
	fmt.Print("\033[1;1H")   // 移动到顶部
	fmt.Println()            // 第1行留给计数器
	fmt.Println()
	fmt.Println("----------------------------------------")
	fmt.Println("测试: 在下方输入文字，观察是否有延迟")
	fmt.Println("输入 'quit' 退出")
	fmt.Println("----------------------------------------")
	fmt.Print("\n> ")

	// 读取用户输入
	scanner := bufio.NewScanner(os.Stdin)

	go func() {
		for scanner.Scan() {
			text := strings.TrimSpace(scanner.Text())
			inputCh <- text
		}
	}()

	for {
		select {
		case text := <-inputCh:
			if text == "quit" || text == "exit" {
				close(stopCh)
				fmt.Println("\n\n再见!")
				return
			}

			// 显示用户输入（保存/恢复光标以避免干扰）
			fmt.Printf("\033[s")
			fmt.Printf("\033[8;1H")   // 移动到第8行
			fmt.Printf("\033[K")
			fmt.Printf("你输入了: %s (时间: %s)", text, time.Now().Format("15:04:05.000"))
			fmt.Printf("\033[u")
			fmt.Print("\n> ")
		}
	}
}
