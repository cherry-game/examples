package test_goroutine

import (
	"context"
	"log"
	"sync"
	"testing"
	"time"
)

type TaskFunc func(ctx context.Context)

func runTaskFunc(wg *sync.WaitGroup, ctx context.Context, taskName string, f TaskFunc) {
	defer wg.Done()

	log.Printf("Task %s start!\n", taskName)
	f(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Task %s revice exit signal \n", taskName)
			return
		}
	}
}

func TestStop2(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ctxb, cancelb := context.WithCancel(ctx)
	ctxe, cancele := context.WithCancel(ctx)

	wg := sync.WaitGroup{}

	// 任务树 A->b->C->D 与 E->F->G 共 7 个任务, 且都是无条件派生,
	// 因此在启动前一次性登记。原先是在每个任务的 goroutine 内部调用 wg.Add(1),
	// 这属于 WaitGroup 的误用: Add 与 Wait 并发执行, go vet 会报
	// "WaitGroup.Add called from inside new goroutine"。
	wg.Add(7)

	go runTaskFunc(&wg, ctx, "A", func(ctx context.Context) {
		go runTaskFunc(&wg, ctxb, "b", func(ctx context.Context) {
			go runTaskFunc(&wg, ctx, "C", func(ctx context.Context) {
				go runTaskFunc(&wg, ctx, "D", func(ctx context.Context) {
				})
			})
		})
	})

	go runTaskFunc(&wg, ctxe, "E", func(ctx context.Context) {
		go runTaskFunc(&wg, ctx, "F", func(ctx context.Context) {
			go runTaskFunc(&wg, ctx, "G", func(ctx context.Context) {

			})
		})
	})

	go func() {
		time.Sleep(3 * time.Second)
		cancele()

		time.Sleep(3 * time.Second)
		cancelb()

		time.Sleep(3 * time.Second)
		cancel()
	}()

	wg.Wait()
}
