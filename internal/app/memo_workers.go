package app

import (
	"context"
	"log"
	"time"

	"vhome/internal/service"
)

const (
	memoEmailPollInterval = time.Minute
	memoMaintenancePeriod = 24 * time.Hour
	// Cover the retention window so reminders are still delivered after a
	// multi-day server outage instead of being silently abandoned.
	memoEmailLookback  = 31 * 24 * time.Hour
	memoEmailBatchSize = 50
)

func (a *APP) startMemoWorkers(ctx context.Context) {
	memoService := a.workers.Memo
	notificationService := a.workers.Notification
	calendarSyncService := a.workers.Calendar

	a.workerGroup.Add(2)

	go func() {
		defer a.workerGroup.Done()

		runMemoEmailDelivery(ctx, memoService, notificationService)
		ticker := time.NewTicker(memoEmailPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runMemoEmailDelivery(ctx, memoService, notificationService)
			}
		}
	}()

	go func() {
		defer a.workerGroup.Done()

		runMemoMaintenance(ctx, memoService, calendarSyncService)
		ticker := time.NewTicker(memoMaintenancePeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runMemoMaintenance(ctx, memoService, calendarSyncService)
			}
		}
	}()
}

func runMemoEmailDelivery(
	parent context.Context,
	memoService *service.MemoService,
	notificationService *service.NotificationService,
) {
	ctx, cancel := context.WithTimeout(parent, 45*time.Second)
	defer cancel()

	now := time.Now()
	memos, err := memoService.PendingEmailMemos(
		ctx,
		now.Add(-memoEmailLookback),
		now,
		memoEmailBatchSize,
	)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("memo email worker: list pending memos: %v", err)
		}
		return
	}

	for _, memo := range memos {
		result, deliveryErr := notificationService.DeliverMemoEmail(ctx, memo)
		if !result.Enabled {
			continue
		}

		if deliveryErr != nil {
			log.Printf(
				"memo email worker: memo_id=%d sent=%d eligible=%d: %v",
				memo.ID,
				result.Sent,
				result.Eligible,
				deliveryErr,
			)
		}

		// 没有填写邮箱的收件人无需反复扫描；部分成功时也停止整条备忘录的
		// 重试，避免已经收到邮件的成员在下一分钟收到重复邮件。
		if result.Eligible == 0 || result.Sent > 0 {
			if err := memoService.MarkEmailSent(ctx, memo, time.Now()); err != nil {
				log.Printf("memo email worker: mark memo_id=%d sent: %v", memo.ID, err)
			}
		}
	}
}

func runMemoMaintenance(
	parent context.Context,
	memoService *service.MemoService,
	calendarSyncService *service.CalendarSyncService,
) {
	ctx, cancel := context.WithTimeout(parent, 90*time.Second)
	defer cancel()

	if _, err := memoService.CleanupMemos(ctx, time.Now()); err != nil && ctx.Err() == nil {
		log.Printf("memo maintenance: cleanup expired memos: %v", err)
	}

	if _, err := calendarSyncService.SyncUpcoming(ctx, time.Now()); err != nil && ctx.Err() == nil {
		// 国务院通常在年末发布下一年度安排。同步失败不会影响项目启动，
		// 已经写入数据库的日历数据仍可继续使用，也可由 owner 手动重试。
		log.Printf("memo maintenance: sync holiday calendar: %v", err)
	}
}
