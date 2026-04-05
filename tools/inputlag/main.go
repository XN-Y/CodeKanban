// 测试终端在快速输出时输入是否卡顿
// 运行: go run ./tools/inputlag
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	stopCh := make(chan struct{})
	inputCh := make(chan string)

	// 快速更新 goroutine - 同时更新30行
	go func() {
		counter := 0
		chars := "░▒▓█▓▒░"

		for {
			select {
			case <-stopCh:
				return
			default:
				counter++
				fmt.Printf("\033[s") // 保存光标

				// 更新前30行
				for i := 1; i <= 30; i++ {
					fmt.Printf("\033[%d;1H\033[K", i) // 移动到第i行并清除

					// 生成动态内容
					offset := (counter + i) % len(chars)
					bar := ""
					for j := 0; j < 60; j++ {
						idx := (offset + j) % len(chars)
						bar += string(chars[idx])
					}
					fmt.Printf("行%02d | %s | %d", i, bar, counter)
				}

				fmt.Printf("\033[u") // 恢复光标
				time.Sleep(16 * time.Millisecond) // ~60 FPS
			}
		}
	}()

	// 设置屏幕
	fmt.Print("\033[2J\033[1;1H")
	fmt.Printf("\033[32;1H") // 移动到第32行
	fmt.Println("----------------------------------------")
	fmt.Println("在上方30行快速更新，在此输入文字测试延迟")
	fmt.Println("输入 quit 退出")
	fmt.Println("----------------------------------------")
	fmt.Print("> ")

	// 输入 goroutine
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			inputCh <- strings.TrimSpace(scanner.Text())
		}
	}()

	for text := range inputCh {
		if text == "quit" || text == "exit" {
			close(stopCh)
			fmt.Println("\n再见!")
			return
		}
		fmt.Printf("\033[s\033[37;1H\033[K你输入了: %s (%s)\033[u\n> ",
			text, time.Now().Format("15:04:05.000"))
	}
}
